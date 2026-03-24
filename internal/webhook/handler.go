package webhook

import (
	"context"
	"fmt"
	"os"

	"github.com/Map130/universities/internal/repository"
	"github.com/gofiber/fiber/v2"
)

// WebhookRepos defines the interface for repositories needed during sync
type WebhookRepos interface {
	Universities() repository.UniversityRepository
	Specialties() repository.SpecialtyRepository
	Groups() repository.SpecialtyGroupRepository
	Subjects() repository.SubjectRepository
}

// SyncResults holds the result of a sync operation
type SyncResults struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Errors  []string `json:"errors"`
}

// WebhookHandler returns a fiber.Handler for GitHub push webhook events
func WebhookHandler(repos WebhookRepos, secret string, gitClient *GitClient) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Verify HMAC-SHA256 signature
		signature := c.Get("X-Hub-Signature-256")
		if !VerifySignature(c.Body(), signature, secret) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "invalid signature"})
		}

		// 2. Parse push event -> list of changed files
		event, err := ParsePushEvent(c.Body())
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
		}

		// 3. For each file: fetch content -> validate -> upsert/delete
		results := syncChangedFiles(c.Context(), event, repos, gitClient)

		return c.JSON(fiber.Map{
			"processed": results.Total,
			"success":   results.Success,
			"errors":    results.Errors,
		})
	}
}

// FullSyncHandler returns a fiber.Handler for full sync operations
func FullSyncHandler(repos WebhookRepos, gitClient *GitClient) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Check API-key
		if c.Get("X-API-Key") != os.Getenv("WEBHOOK_SECRET") {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "unauthorized"})
		}

		// 2. Fetch all files from data repository
		files, err := gitClient.ListAllDataFiles(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list files"})
		}

		// 3. Sync each file
		results := syncAllFiles(c.Context(), files, repos, gitClient)
		return c.JSON(results)
	}
}

// syncChangedFiles processes a push event and syncs changed files
func syncChangedFiles(ctx context.Context, event *PushEvent, repos WebhookRepos, gitClient *GitClient) SyncResults {
	// TODO: Implement file sync logic
	// 1. Extract added/modified/removed files from event.Commits
	// 2. Deduplicate files
	// 3. Fetch content using GitClient for added/modified files
	// 4. Validate against JSON schemas
	// 5. Update or Delete in database using WebhookRepos

	return SyncResults{
		Total:   0,
		Success: 0,
		Errors:  []string{},
	}
}

// syncAllFiles processes a full sync of all files
func syncAllFiles(ctx context.Context, files []string, repos WebhookRepos, gitClient *GitClient) SyncResults {
	results := SyncResults{
		Total:   len(files),
		Success: 0,
		Errors:  []string{},
	}

	for _, file := range files {
		content, err := gitClient.FetchFileContent(ctx, file)
		if err != nil {
			results.Errors = append(results.Errors, fmt.Sprintf("failed to fetch %s: %v", file, err))
			continue
		}

		err = processFile(ctx, file, content, repos)
		if err != nil {
			results.Errors = append(results.Errors, fmt.Sprintf("failed to process %s: %v", file, err))
		} else {
			results.Success++
		}
	}

	return results
}
