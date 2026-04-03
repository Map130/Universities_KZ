package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Map130/universities/internal/models"
	"github.com/Map130/universities/internal/repository"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

type mockRepos struct {
	uni     *mockUniRepo
	spec    *mockSpecRepo
	group   *mockGroupRepo
	subject *mockSubjectRepo
}

func (m *mockRepos) Universities() repository.UniversityRepository { return m.uni }
func (m *mockRepos) Specialties() repository.SpecialtyRepository   { return m.spec }
func (m *mockRepos) Groups() repository.SpecialtyGroupRepository   { return m.group }
func (m *mockRepos) Subjects() repository.SubjectRepository        { return m.subject }

type mockUniRepo struct {
	repository.UniversityRepository
	deleted map[string]bool
	updated map[string]bool
}

func (m *mockUniRepo) Update(ctx context.Context, id surrealmodels.RecordID, u models.University) (*models.University, error) {
	m.updated[id.ID.(string)] = true
	return &u, nil
}
func (m *mockUniRepo) DeleteAllOffers(ctx context.Context, universityID surrealmodels.RecordID) error {
	return nil
}
func (m *mockUniRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error {
	m.deleted[id.ID.(string)] = true
	return nil
}
func (m *mockUniRepo) CreateOffer(ctx context.Context, universityID, specialtyID surrealmodels.RecordID, input models.CreateOfferInput) (*models.Offers, error) {
	return nil, nil
}

type mockSpecRepo struct {
	repository.SpecialtyRepository
}
func (m *mockSpecRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error { return nil }

type mockGroupRepo struct {
	repository.SpecialtyGroupRepository
}
func (m *mockGroupRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error { return nil }

type mockSubjectRepo struct {
	repository.SubjectRepository
}
func (m *mockSubjectRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error { return nil }

func TestSyncChangedFiles_IgnoredBranch(t *testing.T) {
	client := NewGitClient("token", "owner", "repo", "main")
	event := &PushEvent{
		Ref:     "refs/heads/dev",
		Commits: []Commit{{Added: []string{"data/universities/test.yml"}}},
	}

	repos := &mockRepos{}

	results := syncChangedFiles(context.Background(), event, repos, client)

	if results.Total != 0 {
		t.Errorf("Expected 0 total, got %d", results.Total)
	}
	if len(results.Errors) == 0 || results.Errors[0] != "ignored push to refs/heads/dev, expected refs/heads/main" {
		t.Errorf("Expected branch ignore error, got %v", results.Errors)
	}
}

type rewriteTransport struct {
	URL   string
	Proxy http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	testReq := req.Clone(req.Context())
	testReq.URL.Scheme = "http"
	testReq.URL.Host = "test"
	testReq.URL, _ = testReq.URL.Parse(t.URL + req.URL.Path)
	return t.Proxy.RoundTrip(testReq)
}

func TestSyncChangedFiles_ProcessAddedAndRemoved(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/owner/repo/main/data/universities/added.yml" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`
name:
  ru: "Тестовый ВУЗ"
abbr: "TEST"
city: "Almaty"
type: "national"
`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewGitClient("token", "owner", "repo", "main")
	client.HTTPClient = ts.Client()
	client.HTTPClient.Transport = &rewriteTransport{URL: ts.URL, Proxy: http.DefaultTransport}

	event := &PushEvent{
		Ref: "refs/heads/main",
		Commits: []Commit{
			{
				Added:    []string{"data/universities/added.yml"},
				Removed:  []string{"data/universities/removed.yml"},
				Modified: []string{"ignored_file.txt"},
			},
		},
	}

	uniRepo := &mockUniRepo{deleted: make(map[string]bool), updated: make(map[string]bool)}
	repos := &mockRepos{
		uni:     uniRepo,
		spec:    &mockSpecRepo{},
		group:   &mockGroupRepo{},
		subject: &mockSubjectRepo{},
	}

	results := syncChangedFiles(context.Background(), event, repos, client)
	t.Logf("Total: %d, Success: %d, Errors: %v", results.Total, results.Success, results.Errors)

	if results.Total != 2 {
		t.Errorf("Expected 2 total processed files (1 added, 1 removed), got %d. Errors: %v", results.Total, results.Errors)
	}
	if results.Success != 2 {
		t.Errorf("Expected 2 successful operations, got %d. Errors: %v", results.Success, results.Errors)
	}

	if !uniRepo.updated["added"] {
		t.Errorf("Expected 'added' university to be updated/created")
	}

	if !uniRepo.deleted["removed"] {
		t.Errorf("Expected 'removed' university to be deleted")
	}
}
