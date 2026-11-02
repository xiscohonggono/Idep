package handler

import (
	"api/internal/domain"
	"api/internal/repository"
	"api/internal/service"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
)

const chapterContent = "Plants use sunlight to make food."

var (
	caller    = uuid.New()
	chapterID = uuid.New()
	versionID = uuid.New()
)

// analogyRepo embeds the nil-pool repository, which answers ErrDatabaseUnavailable
// for anything a test does not override.
type analogyRepo struct {
	*repository.PostgresAnalogyRepository
	calls       int64
	explanation *domain.AIExplanation
	highlight   *domain.StudyHighlight
}

func newAnalogyRepo() *analogyRepo {
	return &analogyRepo{PostgresAnalogyRepository: repository.NewPostgresAnalogyRepository(nil)}
}

func (r *analogyRepo) UserExists(context.Context, uuid.UUID) (bool, error) { return true, nil }

func (r *analogyRepo) GetPublishedChapterVersion(_ context.Context, chapter, _ uuid.UUID) (*domain.ChapterVersion, error) {
	if chapter != chapterID {
		return nil, domain.ErrChapterNotFound
	}
	return &domain.ChapterVersion{ChapterID: chapterID, VersionID: versionID, ContentMarkdown: chapterContent}, nil
}

func (r *analogyRepo) ListUserWorkExperiences(context.Context, uuid.UUID) ([]domain.WorkExperience, error) {
	return nil, nil
}

func (r *analogyRepo) ListPublishedCatalog(context.Context) ([]domain.CatalogChapter, error) {
	return nil, nil
}

func (r *analogyRepo) CountRecentAICalls(context.Context, uuid.UUID) (int64, error) {
	return r.calls, nil
}

func (r *analogyRepo) FindHighlight(context.Context, domain.StudyHighlight) (*domain.StudyHighlight, error) {
	return r.highlight, nil
}

func (r *analogyRepo) CreateHighlight(_ context.Context, h domain.StudyHighlight) (*domain.StudyHighlight, error) {
	h.ID = uuid.New()
	return &h, nil
}

func (r *analogyRepo) GetHighlight(context.Context, uuid.UUID) (*domain.StudyHighlight, error) {
	if r.highlight == nil {
		return nil, domain.ErrHighlightNotFound
	}
	return r.highlight, nil
}

func (r *analogyRepo) FindExplanation(context.Context, uuid.UUID, *string) (*domain.AIExplanation, error) {
	return r.explanation, nil
}

func (r *analogyRepo) CreateExplanation(_ context.Context, e domain.AIExplanation) (*domain.AIExplanation, error) {
	e.ID = uuid.New()
	return &e, nil
}

func (r *analogyRepo) ListElaborations(context.Context, uuid.UUID) ([]domain.AIElaboration, error) {
	return []domain.AIElaboration{}, nil
}

func (r *analogyRepo) GetUserFeedback(context.Context, uuid.UUID, uuid.UUID) (*domain.ExplanationFeedback, error) {
	return nil, nil
}

func (r *analogyRepo) GetExplanation(context.Context, uuid.UUID) (*domain.AIExplanation, error) {
	if r.explanation == nil {
		return nil, domain.ErrExplanationNotFound
	}
	return r.explanation, nil
}

func (r *analogyRepo) CreateElaboration(_ context.Context, _ uuid.UUID, e domain.AIElaboration) (*domain.AIElaboration, error) {
	e.ID = uuid.New()
	return &e, nil
}

func (r *analogyRepo) UpsertFeedback(_ context.Context, f domain.ExplanationFeedback) (*domain.ExplanationFeedback, error) {
	f.ID = uuid.New()
	return &f, nil
}

type analogyGenerator struct {
	explanation *domain.GeneratedExplanation
	elaboration *domain.GeneratedElaboration
	err         error
}

func (g analogyGenerator) GenerateExplanation(context.Context, domain.GenerationRequest) (*domain.GeneratedExplanation, error) {
	return g.explanation, g.err
}

func (g analogyGenerator) GenerateElaboration(context.Context, domain.ElaborationRequest) (*domain.GeneratedElaboration, error) {
	return g.elaboration, g.err
}

func completeExplanation() *domain.GeneratedExplanation {
	return &domain.GeneratedExplanation{
		AnalogySteps:  []domain.AnalogyStep{{StepNumber: 1, WorkElement: "w", SubjectTerm: "s", Explanation: "e"}},
		AnalogyLimits: []domain.AnalogyLimit{{AnalogyLimit: "l", AccurateExplanation: "a"}},
	}
}

func explanationBody(text string) map[string]any {
	return map[string]any{"version_id": versionID, "selected_text": text, "start_offset": 0, "end_offset": len(chapterContent)}
}

func explanationsPath(chapter uuid.UUID) string {
	return "/api/v1/chapters/" + chapter.String() + "/explanations"
}

func header(user string) string { return "X-User-ID: " + user }

func TestCreateExplanationStatus(t *testing.T) {
	cached := &domain.AIExplanation{ID: uuid.New(), UserID: caller, Terms: []domain.TermDefinition{}, Prerequisites: []domain.PrerequisiteLink{}}
	highlight := &domain.StudyHighlight{ID: uuid.New(), UserID: caller, SelectedText: chapterContent}

	tests := []struct {
		name      string
		repo      *analogyRepo
		generator analogyGenerator
		user      string
		chapter   uuid.UUID
		text      string
		status    int
	}{
		{"created", newAnalogyRepo(), analogyGenerator{explanation: completeExplanation()}, caller.String(), chapterID, chapterContent, http.StatusCreated},
		{"cached", &analogyRepo{PostgresAnalogyRepository: repository.NewPostgresAnalogyRepository(nil), explanation: cached, highlight: highlight}, analogyGenerator{}, caller.String(), chapterID, chapterContent, http.StatusOK},
		{"short selection", newAnalogyRepo(), analogyGenerator{}, caller.String(), chapterID, "Plants", http.StatusUnprocessableEntity},
		{"too many paragraphs", newAnalogyRepo(), analogyGenerator{}, caller.String(), chapterID, strings.Repeat("Some paragraph.\n\n", 4), http.StatusUnprocessableEntity},
		{"not verbatim", newAnalogyRepo(), analogyGenerator{}, caller.String(), chapterID, "Animals eat their food.", http.StatusUnprocessableEntity},
		{"unknown chapter", newAnalogyRepo(), analogyGenerator{}, caller.String(), uuid.New(), chapterContent, http.StatusNotFound},
		{"quota", &analogyRepo{PostgresAnalogyRepository: repository.NewPostgresAnalogyRepository(nil), calls: domain.MaxAICallsPerDay}, analogyGenerator{}, caller.String(), chapterID, chapterContent, http.StatusTooManyRequests},
		{"provider down", newAnalogyRepo(), analogyGenerator{err: domain.ErrAIUnavailable}, caller.String(), chapterID, chapterContent, http.StatusServiceUnavailable},
		{"no limits", newAnalogyRepo(), analogyGenerator{explanation: &domain.GeneratedExplanation{}}, caller.String(), chapterID, chapterContent, http.StatusServiceUnavailable},
		{"invalid user", newAnalogyRepo(), analogyGenerator{}, "not-a-uuid", chapterID, chapterContent, http.StatusUnauthorized},
		{"nil user", newAnalogyRepo(), analogyGenerator{}, uuid.Nil.String(), chapterID, chapterContent, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, api := humatest.New(t)
			RegisterAnalogyRoutes(api, service.NewAnalogyService(tt.repo, tt.generator))

			resp := api.Post(explanationsPath(tt.chapter), header(tt.user), explanationBody(tt.text))
			if resp.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", resp.Code, tt.status, resp.Body)
			}
		})
	}
}

func TestMissingUserHeaderIsUnauthorized(t *testing.T) {
	_, api := humatest.New(t)
	RegisterAnalogyRoutes(api, service.NewAnalogyService(newAnalogyRepo(), analogyGenerator{}))

	for name, resp := range map[string]int{
		"create":    api.Post(explanationsPath(chapterID), explanationBody(chapterContent)).Code,
		"list":      api.Get("/api/v1/highlights/" + uuid.NewString() + "/explanations").Code,
		"elaborate": api.Post("/api/v1/explanations/"+uuid.NewString()+"/elaborate", map[string]any{"target_section": "general"}).Code,
		"feedback":  api.Put("/api/v1/explanations/"+uuid.NewString()+"/feedback", map[string]any{"rating": "like"}).Code,
	} {
		if resp != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, resp)
		}
	}
}

func TestNilPoolFailsClosed(t *testing.T) {
	_, api := humatest.New(t)
	repo := repository.NewPostgresAnalogyRepository(nil)
	RegisterAnalogyRoutes(api, service.NewAnalogyService(repo, analogyGenerator{}))

	resp := api.Post(explanationsPath(chapterID), header(caller.String()), explanationBody(chapterContent))
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d: %s", resp.Code, resp.Body)
	}
}

func TestListHighlightExplanationsAccess(t *testing.T) {
	path := "/api/v1/highlights/" + uuid.NewString() + "/explanations"

	tests := []struct {
		name      string
		highlight *domain.StudyHighlight
		status    int
	}{
		{"not found", nil, http.StatusNotFound},
		{"another user's highlight", &domain.StudyHighlight{UserID: uuid.New()}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, api := humatest.New(t)
			repo := newAnalogyRepo()
			repo.highlight = tt.highlight
			RegisterAnalogyRoutes(api, service.NewAnalogyService(repo, analogyGenerator{}))

			if resp := api.Get(path, header(caller.String())); resp.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", resp.Code, tt.status, resp.Body)
			}
		})
	}
}

func TestElaborateAndFeedbackRoutes(t *testing.T) {
	hint := "Back to the passage."
	_, api := humatest.New(t)
	repo := newAnalogyRepo()
	repo.explanation = &domain.AIExplanation{ID: uuid.New(), UserID: caller, SelectedText: chapterContent}
	generator := analogyGenerator{elaboration: &domain.GeneratedElaboration{IsOnTopic: false, HintText: &hint, ModelIdentifier: "m"}}
	RegisterAnalogyRoutes(api, service.NewAnalogyService(repo, generator))

	base := "/api/v1/explanations/" + repo.explanation.ID.String()
	user := header(caller.String())

	resp := api.Post(base+"/elaborate", user, map[string]any{"target_section": "general", "user_prompt": "What is the weather?"})
	if resp.Code != http.StatusOK {
		t.Fatalf("elaborate status = %d: %s", resp.Code, resp.Body)
	}
	var elaboration domain.AIElaboration
	if err := json.Unmarshal(resp.Body.Bytes(), &elaboration); err != nil {
		t.Fatal(err)
	}
	if elaboration.IsOnTopic || elaboration.HintText == nil || *elaboration.HintText != hint {
		t.Fatalf("elaboration = %+v", elaboration)
	}

	if resp := api.Post(base+"/elaborate", user, map[string]any{"target_section": "general", "user_prompt": strings.Repeat("x", 301)}); resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("long prompt status = %d", resp.Code)
	}
	if resp := api.Post(base+"/elaborate", user, map[string]any{"target_section": "step"}); resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing index status = %d", resp.Code)
	}
	if resp := api.Post(base+"/elaborate", header(uuid.NewString()), map[string]any{"target_section": "general"}); resp.Code != http.StatusForbidden {
		t.Fatalf("other user status = %d", resp.Code)
	}

	for _, rating := range []string{"like", "dislike"} {
		resp := api.Put(base+"/feedback", user, map[string]any{"rating": rating, "comment": "Felt right"})
		if resp.Code != http.StatusOK {
			t.Fatalf("%s status = %d: %s", rating, resp.Code, resp.Body)
		}
	}
	if resp := api.Put(base+"/feedback", user, map[string]any{"rating": "meh"}); resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad rating status = %d", resp.Code)
	}
	if resp := api.Put(base+"/feedback", user, map[string]any{"rating": "like", "comment": strings.Repeat("x", 501)}); resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("long comment status = %d", resp.Code)
	}
}

func TestUnknownErrorIsNotLeaked(t *testing.T) {
	err := analogyError("test", errors.New("pq: secret detail"))
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("err = %v", err)
	}
}
