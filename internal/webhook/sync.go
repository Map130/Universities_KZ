package webhook

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/Map130/universities/internal/models"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
	"gopkg.in/yaml.v3"
)

// ProcessTarball extracts and processes data files from a GitHub tarball stream.
// It reads the compressed stream on the fly to minimize memory usage and
// routes YAML files to their respective processors based on directory structure.
func ProcessTarball(ctx context.Context, stream io.Reader, repos WebhookRepos) error {
	gzr, err := gzip.NewReader(stream)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Skip directories and focus only on files
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// GitHub tarballs have a dynamic root directory like `owner-repo-commitHash/`
		// We only care about files inside the `data/` directory.
		if !strings.Contains(header.Name, "/data/") {
			continue
		}

		// For now, we only process YAML files automatically.
		// MD files (descriptions) can be linked or parsed separately if needed.
		if !strings.HasSuffix(header.Name, ".yml") && !strings.HasSuffix(header.Name, ".yaml") {
			continue
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			log.Printf("[sync] failed to read file %s: %v", header.Name, err)
			continue
		}

		// Route the file content based on its path
		switch {
		case strings.Contains(header.Name, "/data/universities/"):
			err = processUniversity(ctx, header.Name, content, repos)
		case strings.Contains(header.Name, "/data/specialties/"):
			err = processSpecialty(ctx, header.Name, content, repos)
		case strings.Contains(header.Name, "/data/groups/"):
			err = processGroup(ctx, header.Name, content, repos)
		case strings.Contains(header.Name, "/data/subjects/"):
			err = processSubject(ctx, header.Name, content, repos)
		}

		if err != nil {
			log.Printf("[sync] failed to process %s: %v", header.Name, err)
		}
	}

	return nil
}

func extractID(filename string) string {
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// processFile routes the file content to the appropriate processor based on its path
func processFile(ctx context.Context, filename string, content []byte, repos WebhookRepos) error {
	switch {
	case strings.Contains(filename, "/data/universities/"):
		return processUniversity(ctx, filename, content, repos)
	case strings.Contains(filename, "/data/specialties/"):
		return processSpecialty(ctx, filename, content, repos)
	case strings.Contains(filename, "/data/groups/"):
		return processGroup(ctx, filename, content, repos)
	case strings.Contains(filename, "/data/subjects/"):
		return processSubject(ctx, filename, content, repos)
	default:
		return nil
	}
}

// processUniversity unmarshals the YAML and updates the database.
func processUniversity(ctx context.Context, filename string, data []byte, repos WebhookRepos) error {
	if repos == nil {
		return nil
	}

	type OfferInput struct {
		SpecialtyCode string `yaml:"specialty_code"`
		models.CreateOfferInput `yaml:",inline"`
	}

	type UniversityFile struct {
		models.University `yaml:",inline"`
		Offers            []OfferInput `yaml:"offers"`
	}

	var fileData UniversityFile
	if err := yaml.Unmarshal(data, &fileData); err != nil {
		return fmt.Errorf("failed to unmarshal university %s: %w", filename, err)
	}

	idStr := extractID(filename)
	recordID := surrealmodels.NewRecordID("university", idStr)

	// Upsert university
	_, err := repos.Universities().Update(ctx, recordID, fileData.University)
	if err != nil {
		return fmt.Errorf("failed to update university %s: %w", idStr, err)
	}

	// Recreate offers
	if err := repos.Universities().DeleteAllOffers(ctx, recordID); err != nil {
		return fmt.Errorf("failed to clear offers for university %s: %w", idStr, err)
	}

	for _, offerInput := range fileData.Offers {
		if offerInput.SpecialtyCode == "" {
			continue
		}
		specID := surrealmodels.NewRecordID("specialty", offerInput.SpecialtyCode)
		if _, err := repos.Universities().CreateOffer(ctx, recordID, specID, offerInput.CreateOfferInput); err != nil {
			return fmt.Errorf("failed to create offer for university %s, specialty %s: %w", idStr, offerInput.SpecialtyCode, err)
		}
	}

	return nil
}

func processSpecialty(ctx context.Context, filename string, data []byte, repos WebhookRepos) error {
	if repos == nil {
		return nil
	}

	type SpecialtyFile struct {
		models.Specialty `yaml:",inline"`
		GroupCode        string `yaml:"group_code"`
	}

	var fileData SpecialtyFile
	if err := yaml.Unmarshal(data, &fileData); err != nil {
		return fmt.Errorf("failed to unmarshal specialty %s: %w", filename, err)
	}

	idStr := extractID(filename)
	recordID := surrealmodels.NewRecordID("specialty", idStr)

	if fileData.GroupCode != "" {
		fileData.Group = surrealmodels.NewRecordID("specialty_group", fileData.GroupCode)
	}

	_, err := repos.Specialties().Update(ctx, recordID, fileData.Specialty)
	if err != nil {
		return fmt.Errorf("failed to update specialty %s: %w", idStr, err)
	}

	return nil
}

func processGroup(ctx context.Context, filename string, data []byte, repos WebhookRepos) error {
	if repos == nil {
		return nil
	}

	type SubjectRef struct {
		Code  string `yaml:"code"`
		Order int    `yaml:"order"`
	}

	type GroupFile struct {
		models.SpecialtyGroup `yaml:",inline"`
		Subjects              []SubjectRef `yaml:"subjects"`
	}

	var fileData GroupFile
	if err := yaml.Unmarshal(data, &fileData); err != nil {
		return fmt.Errorf("failed to unmarshal group %s: %w", filename, err)
	}

	idStr := extractID(filename)
	recordID := surrealmodels.NewRecordID("specialty_group", idStr)

	_, err := repos.Groups().Update(ctx, recordID, fileData.SpecialtyGroup)
	if err != nil {
		return fmt.Errorf("failed to update group %s: %w", idStr, err)
	}

	// Recreate requires edges if Subjects array contains valid codes
	if err := repos.Groups().DeleteAllRequires(ctx, recordID); err != nil {
		return fmt.Errorf("failed to clear requires for group %s: %w", idStr, err)
	}

	for _, subj := range fileData.Subjects {
		if subj.Code == "" {
			continue
		}
		subjID := surrealmodels.NewRecordID("subject", subj.Code)
		input := models.CreateRequiresInput{
			Priority: models.SubjectPriority(subj.Order),
		}
		if _, err := repos.Groups().CreateRequires(ctx, recordID, subjID, input); err != nil {
			return fmt.Errorf("failed to create requires for group %s, subject %s: %w", idStr, subj.Code, err)
		}
	}

	return nil
}

func processSubject(ctx context.Context, filename string, data []byte, repos WebhookRepos) error {
	if repos == nil {
		return nil
	}

	var subjects []models.Subject
	if err := yaml.Unmarshal(data, &subjects); err != nil {
		// Fallback for single subject
		var singleSubject models.Subject
		if errSingle := yaml.Unmarshal(data, &singleSubject); errSingle == nil {
			subjects = []models.Subject{singleSubject}
		} else {
			return fmt.Errorf("failed to unmarshal subject %s: %w", filename, err)
		}
	}

	for _, subj := range subjects {
		idStr := subj.Code
		if idStr == "" {
			idStr = extractID(filename)
		}
		recordID := surrealmodels.NewRecordID("subject", idStr)

		_, err := repos.Subjects().Update(ctx, recordID, subj)
		if err != nil {
			return fmt.Errorf("failed to update subject %s: %w", idStr, err)
		}
	}

	return nil
}
