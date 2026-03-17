package webhook

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
)

// ProcessTarball extracts and processes data files from a GitHub tarball stream.
// It reads the compressed stream on the fly to minimize memory usage and
// routes JSON files to their respective processors based on directory structure.
func ProcessTarball(ctx context.Context, stream io.Reader, repos WebhookRepos) error {
	gzr, err := gzip.NewReader(stream)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

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

		// For now, we only process JSON files automatically.
		// MD files (descriptions) can be linked or parsed separately if needed.
		if !strings.HasSuffix(header.Name, ".json") {
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
			err = processUniversity(ctx, content, repos)
		case strings.Contains(header.Name, "/data/specialties/"):
			err = processSpecialty(ctx, content, repos)
		case strings.Contains(header.Name, "/data/groups/"):
			err = processGroup(ctx, content, repos)
		case strings.Contains(header.Name, "/data/subjects/"):
			err = processSubject(ctx, content, repos)
		}

		if err != nil {
			log.Printf("[sync] failed to process %s: %v", header.Name, err)
		}
	}

	return nil
}

// processUniversity unmarshals the JSON and updates the database.
func processUniversity(ctx context.Context, data []byte, repos WebhookRepos) error {
	// TODO: Unmarshal data into models.University and save using repos
	// Example:
	// var uni models.University
	// if err := json.Unmarshal(data, &uni); err != nil { return err }
	// _, err := repos.Universities().Create(ctx, uni)
	// return err
	return nil
}

func processSpecialty(ctx context.Context, data []byte, repos WebhookRepos) error {
	// TODO: Unmarshal data into models.Specialty and save
	return nil
}

func processGroup(ctx context.Context, data []byte, repos WebhookRepos) error {
	// TODO: Unmarshal data into models.SpecialtyGroup and save
	return nil
}

func processSubject(ctx context.Context, data []byte, repos WebhookRepos) error {
	// TODO: Unmarshal data into models.Subject and save
	return nil
}
