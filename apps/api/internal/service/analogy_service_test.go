package service

import (
	"api/internal/domain"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const passage = "Plants use sunlight to make food.\n\nThis process is photosynthesis.\n\nIt happens in the leaves."

var (
	userID    = uuid.New()
	chapterID = uuid.New()
	versionID = uuid.New()
	subjectID = uuid.New()
	otherPub  = uuid.New()
)

// fakeRepo is an in-memory domain.AnalogyRepository.
type fakeRepo struct {
	unknownUser  bool
	recentCalls  int64
	experiences  []domain.WorkExperience
	catalog      []domain.CatalogChapter
	highlights   []domain.StudyHighlight
	explanations []domain.AIExplanation
	elaborations []domain.AIElaboration
	feedback     []domain.ExplanationFeedback
	exemplars    []domain.LikedExemplar
	pitfalls     []string
}

func (f *fakeRepo) UserExists(context.Context, uuid.UUID) (bool, error) { return !f.unknownUser, nil }

func (f *fakeRepo) GetPublishedChapterVersion(_ context.Context, chapter, version uuid.UUID) (*domain.ChapterVersion, error) {
	if chapter != chapterID || version != versionID {
		return nil, domain.ErrChapterNotFound
	}
	return &domain.ChapterVersion{ChapterID: chapter, SubjectID: subjectID, Title: "Plants", VersionID: version, ContentMarkdown: passage}, nil
}

func (f *fakeRepo) ListUserWorkExperiences(context.Context, uuid.UUID) ([]domain.WorkExperience, error) {
	return f.experiences, nil
}

func (f *fakeRepo) GetActiveJobField(_ context.Context, code string) (*domain.JobField, error) {
	if code == "bogus" {
		return nil, domain.ErrInvalidJobField
	}
	return &domain.JobField{Code: code, Name: code}, nil
}

func (f *fakeRepo) ListPublishedCatalog(context.Context) ([]domain.CatalogChapter, error) {
	return f.catalog, nil
}

func (f *fakeRepo) CountRecentAICalls(context.Context, uuid.UUID) (int64, error) {
	return f.recentCalls, nil
}

func (f *fakeRepo) FindHighlight(_ context.Context, h domain.StudyHighlight) (*domain.StudyHighlight, error) {
	for i := range f.highlights {
		if f.highlights[i].SelectedText == h.SelectedText && f.highlights[i].StartOffset == h.StartOffset {
			return &f.highlights[i], nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) CreateHighlight(_ context.Context, h domain.StudyHighlight) (*domain.StudyHighlight, error) {
	h.ID = uuid.New()
	f.highlights = append(f.highlights, h)
	return &h, nil
}

func (f *fakeRepo) GetHighlight(_ context.Context, id uuid.UUID) (*domain.StudyHighlight, error) {
	for i := range f.highlights {
		if f.highlights[i].ID == id {
			return &f.highlights[i], nil
		}
	}
	return nil, domain.ErrHighlightNotFound
}

func (f *fakeRepo) FindExplanation(_ context.Context, highlightID uuid.UUID, code *string) (*domain.AIExplanation, error) {
	for i := range f.explanations {
		e := f.explanations[i]
		if e.HighlightID != highlightID {
			continue
		}
		if (code == nil && e.IsEverydayFallback) || (code != nil && e.JobFieldCode != nil && *code == *e.JobFieldCode) {
			return &e, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) CreateExplanation(_ context.Context, e domain.AIExplanation) (*domain.AIExplanation, error) {
	e.ID = uuid.New()
	f.explanations = append(f.explanations, e)
	return &e, nil
}

func (f *fakeRepo) GetExplanation(_ context.Context, id uuid.UUID) (*domain.AIExplanation, error) {
	for _, e := range f.explanations {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, domain.ErrExplanationNotFound
}

func (f *fakeRepo) ListExplanations(_ context.Context, highlightID uuid.UUID) ([]domain.AIExplanation, error) {
	var found []domain.AIExplanation
	for _, e := range f.explanations {
		if e.HighlightID == highlightID {
			found = append(found, e)
		}
	}
	return found, nil
}

func (f *fakeRepo) ListLikedExemplars(context.Context, uuid.UUID, string) ([]domain.LikedExemplar, error) {
	return f.exemplars, nil
}

func (f *fakeRepo) ListDislikedPitfalls(context.Context, uuid.UUID, string) ([]string, error) {
	return f.pitfalls, nil
}

func (f *fakeRepo) CreateElaboration(_ context.Context, _ uuid.UUID, e domain.AIElaboration) (*domain.AIElaboration, error) {
	e.ID = uuid.New()
	f.elaborations = append(f.elaborations, e)
	return &e, nil
}

func (f *fakeRepo) ListElaborations(context.Context, uuid.UUID) ([]domain.AIElaboration, error) {
	return []domain.AIElaboration{}, nil
}

func (f *fakeRepo) UpsertFeedback(_ context.Context, feedback domain.ExplanationFeedback) (*domain.ExplanationFeedback, error) {
	f.feedback = append(f.feedback, feedback)
	return &feedback, nil
}

func (f *fakeRepo) GetUserFeedback(context.Context, uuid.UUID, uuid.UUID) (*domain.ExplanationFeedback, error) {
	return nil, nil
}

// fakeGenerator records requests and returns its configured reply.
type fakeGenerator struct {
	reply       *domain.GeneratedExplanation
	err         error
	elaboration *domain.GeneratedElaboration
	requests    []domain.GenerationRequest
	elaborated  []domain.ElaborationRequest
}

func (g *fakeGenerator) GenerateExplanation(_ context.Context, req domain.GenerationRequest) (*domain.GeneratedExplanation, error) {
	g.requests = append(g.requests, req)
	return g.reply, g.err
}

func (g *fakeGenerator) GenerateElaboration(_ context.Context, req domain.ElaborationRequest) (*domain.GeneratedElaboration, error) {
	g.elaborated = append(g.elaborated, req)
	return g.elaboration, g.err
}

func goodReply() *domain.GeneratedExplanation {
	return &domain.GeneratedExplanation{
		AnalogySteps:    []domain.AnalogyStep{{StepNumber: 1, WorkElement: "Oven", SubjectTerm: "Leaf", Explanation: "Heat cooks"}},
		AnalogyLimits:   []domain.AnalogyLimit{{AnalogyLimit: "Ovens need gas", AccurateExplanation: "Leaves use light"}},
		ModelIdentifier: "test-model",
		PromptVersion:   "v1",
	}
}

func input() CreateExplanationInput {
	return CreateExplanationInput{
		UserID:       userID,
		ChapterID:    chapterID,
		VersionID:    versionID,
		SelectedText: "Plants use sunlight to make food.",
		StartOffset:  0,
		EndOffset:    33,
	}
}

func ptr[T any](v T) *T { return &v }

func cook() domain.WorkExperience {
	return domain.WorkExperience{JobFieldCode: "fnb", RoleTitle: ptr("line cook"), JobFieldName: "Food"}
}

func nurse() domain.WorkExperience {
	return domain.WorkExperience{JobFieldCode: "healthcare", JobFieldName: "Healthcare"}
}

func TestSelectionValidation(t *testing.T) {
	fourParagraphs := "One paragraph.\n\nTwo paragraph.\n\nThree paragraph.\n\nFour paragraph."
	tests := []struct {
		name   string
		mutate func(*CreateExplanationInput)
		want   error
	}{
		{"too short", func(in *CreateExplanationInput) { in.SelectedText = "Plants" }, domain.ErrSelectionTooLong},
		{"too long", func(in *CreateExplanationInput) { in.SelectedText = strings.Repeat("x", 3001) }, domain.ErrSelectionTooLong},
		{"four paragraphs", func(in *CreateExplanationInput) { in.SelectedText = fourParagraphs }, domain.ErrSelectionTooLong},
		{"four paragraphs split by spaced blank lines", func(in *CreateExplanationInput) {
			in.SelectedText = strings.ReplaceAll(fourParagraphs, "\n\n", "\n \n")
		}, domain.ErrSelectionTooLong},
		{"not verbatim", func(in *CreateExplanationInput) { in.SelectedText = "Animals eat food to get energy." }, domain.ErrSelectionNotInChapter},
		{"negative start", func(in *CreateExplanationInput) { in.StartOffset = -1 }, domain.ErrSelectionNotInChapter},
		{"end before start", func(in *CreateExplanationInput) { in.EndOffset = 0 }, domain.ErrSelectionNotInChapter},
		{"end past chapter", func(in *CreateExplanationInput) { in.EndOffset = 1 << 40 }, domain.ErrSelectionNotInChapter},
		{"unknown chapter", func(in *CreateExplanationInput) { in.ChapterID = uuid.New() }, domain.ErrChapterNotFound},
		{"unknown job field", func(in *CreateExplanationInput) { in.JobFieldCode = ptr("bogus") }, domain.ErrInvalidJobField},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &fakeGenerator{reply: goodReply()}
			in := input()
			tt.mutate(&in)

			_, _, err := NewAnalogyService(&fakeRepo{}, generator).CreateExplanation(context.Background(), in)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if len(generator.requests) != 0 {
				t.Fatal("the LLM was called for a rejected selection")
			}
		})
	}
}

func TestThreeParagraphsAccepted(t *testing.T) {
	in := input()
	in.SelectedText, in.EndOffset = passage, len(passage)

	saved, created, err := NewAnalogyService(&fakeRepo{}, &fakeGenerator{reply: goodReply()}).CreateExplanation(context.Background(), in)
	if err != nil || !created {
		t.Fatalf("created = %v, err = %v", created, err)
	}
	if !saved.IsEverydayFallback || saved.JobFieldCode != nil {
		t.Fatalf("no saved professions must use the everyday fallback: %+v", saved)
	}
}

func TestUnknownUserNeverReachesLLM(t *testing.T) {
	generator := &fakeGenerator{reply: goodReply()}
	_, _, err := NewAnalogyService(&fakeRepo{unknownUser: true}, generator).CreateExplanation(context.Background(), input())
	if !errors.Is(err, domain.ErrUnauthorized) || len(generator.requests) != 0 {
		t.Fatalf("err = %v, calls = %d", err, len(generator.requests))
	}
}

func TestBestFitAndComparison(t *testing.T) {
	repo := &fakeRepo{experiences: []domain.WorkExperience{cook(), nurse()}}
	reply := goodReply()
	reply.SelectedJobFieldCode = ptr("healthcare")
	generator := &fakeGenerator{reply: reply}
	svc := NewAnalogyService(repo, generator)
	ctx := context.Background()

	first, created, err := svc.CreateExplanation(ctx, input())
	if err != nil || !created {
		t.Fatalf("created = %v, err = %v", created, err)
	}
	if first.JobFieldCode == nil || *first.JobFieldCode != "healthcare" {
		t.Fatalf("job field = %v, want the LLM's healthcare pick", first.JobFieldCode)
	}
	if got := len(generator.requests[0].Professions); got != 2 {
		t.Fatalf("generator saw %d professions, want 2", got)
	}

	again, created, err := svc.CreateExplanation(ctx, input())
	if err != nil || created || again.ID != first.ID || len(generator.requests) != 1 {
		t.Fatalf("repeat request must reuse the saved row: created = %v, err = %v, calls = %d", created, err, len(generator.requests))
	}

	generator.reply = goodReply()
	compare := input()
	compare.JobFieldCode = ptr("fnb")
	second, created, err := svc.CreateExplanation(ctx, compare)
	if err != nil || !created {
		t.Fatalf("created = %v, err = %v", created, err)
	}
	if second.HighlightID != first.HighlightID || second.ID == first.ID {
		t.Fatalf("comparison must add a row to the same highlight: %v %v", second.HighlightID, first.HighlightID)
	}
	if len(repo.highlights) != 1 || len(repo.explanations) != 2 {
		t.Fatalf("%d highlights, %d explanations", len(repo.highlights), len(repo.explanations))
	}
	if second.RoleTitleSnapshot == nil || *second.RoleTitleSnapshot != "line cook" {
		t.Fatalf("role snapshot = %v", second.RoleTitleSnapshot)
	}
}

func TestUnlistedProfessionFallsBackToFirst(t *testing.T) {
	reply := goodReply()
	reply.SelectedJobFieldCode = ptr("astronaut")
	repo := &fakeRepo{experiences: []domain.WorkExperience{cook(), nurse()}}

	saved, _, err := NewAnalogyService(repo, &fakeGenerator{reply: reply}).CreateExplanation(context.Background(), input())
	if err != nil {
		t.Fatal(err)
	}
	if *saved.JobFieldCode != "fnb" {
		t.Fatalf("job field = %s, want the top-priority fnb", *saved.JobFieldCode)
	}
}

func TestCachedExplanationIgnoresQuota(t *testing.T) {
	repo := &fakeRepo{experiences: []domain.WorkExperience{cook()}}
	generator := &fakeGenerator{reply: goodReply()}
	svc := NewAnalogyService(repo, generator)
	ctx := context.Background()

	if _, _, err := svc.CreateExplanation(ctx, input()); err != nil {
		t.Fatal(err)
	}
	repo.recentCalls = domain.MaxAICallsPerDay
	if _, created, err := svc.CreateExplanation(ctx, input()); err != nil || created {
		t.Fatalf("created = %v, err = %v", created, err)
	}
}

func TestRateLimit(t *testing.T) {
	generator := &fakeGenerator{reply: goodReply()}
	repo := &fakeRepo{recentCalls: domain.MaxAICallsPerDay}

	_, _, err := NewAnalogyService(repo, generator).CreateExplanation(context.Background(), input())
	if !errors.Is(err, domain.ErrAIRateLimitExceeded) || len(generator.requests) != 0 {
		t.Fatalf("err = %v, calls = %d", err, len(generator.requests))
	}

	repo.recentCalls = domain.MaxAICallsPerDay - 1
	if _, _, err := NewAnalogyService(repo, generator).CreateExplanation(context.Background(), input()); err != nil {
		t.Fatalf("call 30 must pass: %v", err)
	}
}

func TestHallucinatedLinksAreStripped(t *testing.T) {
	fake := uuid.New()
	current := domain.CatalogChapter{ID: chapterID, Slug: "plants", Title: "Plants"}
	published := domain.CatalogChapter{ID: otherPub, Slug: "cells", Title: "Cells"}

	reply := goodReply()
	reply.Terms = []domain.TermDefinition{
		{Term: "real", Definition: "d", LinkedChapterID: &otherPub, LinkedChapterSlug: ptr("wrong"), LinkedChapterTitle: ptr("Wrong")},
		{Term: "fake", Definition: "d", LinkedChapterID: &fake, LinkedChapterSlug: ptr("ghost"), LinkedChapterTitle: ptr("Ghost")},
	}
	reply.Prerequisites = []domain.PrerequisiteLink{
		{ChapterID: otherPub, Slug: "wrong", Title: "Wrong", Reason: "r"},
		{ChapterID: fake, Slug: "ghost", Title: "Ghost", Reason: "r"},
		{ChapterID: chapterID, Slug: "plants", Title: "Plants", Reason: "self"},
	}

	repo := &fakeRepo{catalog: []domain.CatalogChapter{current, published}}
	saved, _, err := NewAnalogyService(repo, &fakeGenerator{reply: reply}).CreateExplanation(context.Background(), input())
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range []domain.AIExplanation{*saved, repo.explanations[0]} {
		if e.Terms[0].LinkedChapterSlug == nil || *e.Terms[0].LinkedChapterSlug != "cells" {
			t.Fatalf("published term link was not corrected: %+v", e.Terms[0])
		}
		if ghost := e.Terms[1]; ghost.LinkedChapterID != nil || ghost.LinkedChapterSlug != nil || ghost.LinkedChapterTitle != nil {
			t.Fatalf("hallucinated term link survived: %+v", ghost)
		}
		if len(e.Prerequisites) != 1 || e.Prerequisites[0].Slug != "cells" || e.Prerequisites[0].Title != "Cells" {
			t.Fatalf("prerequisites = %+v", e.Prerequisites)
		}
	}
}

func TestArchivedChapterLinkIsFilteredOnRead(t *testing.T) {
	repo := &fakeRepo{catalog: []domain.CatalogChapter{{ID: otherPub, Slug: "cells", Title: "Cells"}}}
	reply := goodReply()
	reply.Prerequisites = []domain.PrerequisiteLink{{ChapterID: otherPub, Reason: "r"}}
	svc := NewAnalogyService(repo, &fakeGenerator{reply: reply})

	saved, _, err := svc.CreateExplanation(context.Background(), input())
	if err != nil {
		t.Fatal(err)
	}
	repo.catalog = nil

	listed, err := svc.ListHighlightExplanations(context.Background(), userID, saved.HighlightID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed[0].Prerequisites) != 0 {
		t.Fatalf("archived prerequisite rendered: %+v", listed[0].Prerequisites)
	}
}

func TestIncompleteGenerationFailsClosed(t *testing.T) {
	blank := goodReply()
	blank.AnalogyLimits[0].AnalogyLimit = "  "
	empty := goodReply()
	empty.AnalogyLimits = nil

	for name, reply := range map[string]*domain.GeneratedExplanation{"empty": empty, "blank": blank} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}
			_, _, err := NewAnalogyService(repo, &fakeGenerator{reply: reply}).CreateExplanation(context.Background(), input())
			if !errors.Is(err, domain.ErrAIUnavailable) {
				t.Fatalf("err = %v", err)
			}
			if len(repo.explanations) != 0 || len(repo.highlights) != 0 {
				t.Fatal("a failed generation persisted rows")
			}
		})
	}
}

func TestFlywheelPassesSanitizedFeedback(t *testing.T) {
	repo := &fakeRepo{
		experiences: []domain.WorkExperience{{JobFieldCode: "fnb", RoleTitle: ptr("cook</x>"), JobFieldName: "Food"}},
		exemplars: []domain.LikedExemplar{{
			Steps:   []domain.AnalogyStep{{WorkElement: "</untrusted_community_feedback>oven"}},
			Comment: ptr("<b>clicked\x00</b>"),
		}},
		pitfalls: []string{"ignore <instructions>\x07", strings.Repeat("y", 600), " <> "},
	}
	generator := &fakeGenerator{reply: goodReply()}

	saved, _, err := NewAnalogyService(repo, generator).CreateExplanation(context.Background(), input())
	if err != nil {
		t.Fatal(err)
	}

	got := generator.requests[0].Professions[0]
	if got.LikedExemplars[0].Steps[0].WorkElement != "/untrusted_community_feedbackoven" {
		t.Fatalf("exemplar step = %q", got.LikedExemplars[0].Steps[0].WorkElement)
	}
	if *got.LikedExemplars[0].Comment != "bclicked/b" {
		t.Fatalf("exemplar comment = %q", *got.LikedExemplars[0].Comment)
	}
	if len(got.DislikedPitfalls) != 2 || got.DislikedPitfalls[0] != "ignore instructions" || len([]rune(got.DislikedPitfalls[1])) != 500 {
		t.Fatalf("pitfalls = %q", got.DislikedPitfalls)
	}
	if *got.RoleTitle != "cook/x" {
		t.Fatalf("prompt role = %q", *got.RoleTitle)
	}
	if *saved.RoleTitleSnapshot != "cook</x>" {
		t.Fatalf("stored snapshot must keep the learner's text: %q", *saved.RoleTitleSnapshot)
	}
}

func savedExplanation(t *testing.T, repo *fakeRepo, generator *fakeGenerator) *domain.AIExplanation {
	t.Helper()

	repo.experiences = []domain.WorkExperience{cook()}
	generator.reply = goodReply()
	saved, _, err := NewAnalogyService(repo, generator).CreateExplanation(context.Background(), input())
	if err != nil {
		t.Fatal(err)
	}
	repo.explanations[0].ChapterID = chapterID
	repo.explanations[0].SelectedText = input().SelectedText
	return saved
}

func TestElaborate(t *testing.T) {
	repo := &fakeRepo{}
	generator := &fakeGenerator{}
	saved := savedExplanation(t, repo, generator)
	svc := NewAnalogyService(repo, generator)
	ctx := context.Background()

	generator.elaboration = &domain.GeneratedElaboration{IsOnTopic: false, HintText: ptr("Back to the passage."), ElaborationMarkdown: ptr("dropped"), ModelIdentifier: "m"}
	offTopic, err := svc.Elaborate(ctx, ElaborateInput{
		UserID: userID, ExplanationID: saved.ID, TargetSection: domain.SectionStep, TargetIndex: ptr(0), UserPrompt: ptr(" <what> is the weather? "),
	})
	if err != nil {
		t.Fatal(err)
	}
	if offTopic.IsOnTopic || offTopic.HintText == nil || offTopic.ElaborationMarkdown != nil {
		t.Fatalf("off-topic reply = %+v", offTopic)
	}
	if *offTopic.UserPrompt != "<what> is the weather?" {
		t.Fatalf("stored prompt = %q", *offTopic.UserPrompt)
	}
	if got := generator.elaborated[0]; *got.UserPrompt != "what is the weather?" || !strings.Contains(got.TargetContext, "Oven") {
		t.Fatalf("generator request = %+v", got)
	}

	generator.elaboration = &domain.GeneratedElaboration{IsOnTopic: true, ElaborationMarkdown: ptr("More."), WorkExample: ptr("Oven."), ModelIdentifier: "m"}
	onTopic, err := svc.Elaborate(ctx, ElaborateInput{UserID: userID, ExplanationID: saved.ID, TargetSection: domain.SectionGeneral, UserPrompt: ptr("  ")})
	if err != nil {
		t.Fatal(err)
	}
	if !onTopic.IsOnTopic || onTopic.UserPrompt != nil {
		t.Fatalf("on-topic reply = %+v", onTopic)
	}

	generator.elaboration = &domain.GeneratedElaboration{IsOnTopic: true}
	if _, err := svc.Elaborate(ctx, ElaborateInput{UserID: userID, ExplanationID: saved.ID, TargetSection: domain.SectionGeneral}); !errors.Is(err, domain.ErrAIUnavailable) {
		t.Fatalf("empty elaboration: err = %v", err)
	}
}

func TestElaborateRejections(t *testing.T) {
	repo := &fakeRepo{}
	generator := &fakeGenerator{}
	saved := savedExplanation(t, repo, generator)
	svc := NewAnalogyService(repo, generator)

	tests := []struct {
		name string
		in   ElaborateInput
		want error
	}{
		{"prompt over 300", ElaborateInput{UserID: userID, ExplanationID: saved.ID, TargetSection: "general", UserPrompt: ptr(strings.Repeat("x", 301))}, domain.ErrInvalidElaborationInput},
		{"step without index", ElaborateInput{UserID: userID, ExplanationID: saved.ID, TargetSection: "step"}, domain.ErrInvalidElaborationInput},
		{"index out of range", ElaborateInput{UserID: userID, ExplanationID: saved.ID, TargetSection: "limit", TargetIndex: ptr(9)}, domain.ErrInvalidElaborationInput},
		{"general with index", ElaborateInput{UserID: userID, ExplanationID: saved.ID, TargetSection: "general", TargetIndex: ptr(0)}, domain.ErrInvalidElaborationInput},
		{"another user", ElaborateInput{UserID: uuid.New(), ExplanationID: saved.ID, TargetSection: "general"}, domain.ErrForbidden},
		{"missing explanation", ElaborateInput{UserID: userID, ExplanationID: uuid.New(), TargetSection: "general"}, domain.ErrExplanationNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Elaborate(context.Background(), tt.in); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}

	repo.recentCalls = domain.MaxAICallsPerDay
	if _, err := svc.Elaborate(context.Background(), ElaborateInput{UserID: userID, ExplanationID: saved.ID, TargetSection: "general"}); !errors.Is(err, domain.ErrAIRateLimitExceeded) {
		t.Fatalf("quota: err = %v", err)
	}
	if len(generator.elaborated) != 0 {
		t.Fatal("the LLM was called for a rejected elaboration")
	}
}

func TestFeedback(t *testing.T) {
	repo := &fakeRepo{}
	generator := &fakeGenerator{}
	saved := savedExplanation(t, repo, generator)
	svc := NewAnalogyService(repo, generator)
	ctx := context.Background()

	for _, rating := range []string{"like", "dislike"} {
		got, err := svc.UpsertFeedback(ctx, FeedbackInput{UserID: userID, ExplanationID: saved.ID, Rating: rating, Comment: ptr(" Clicked ")})
		if err != nil {
			t.Fatal(err)
		}
		if got.Rating != rating || *got.Comment != "Clicked" || got.ChapterID != chapterID || *got.JobFieldCode != "fnb" {
			t.Fatalf("feedback = %+v", got)
		}
	}
	if _, err := svc.UpsertFeedback(ctx, FeedbackInput{UserID: userID, ExplanationID: saved.ID, Rating: "like"}); err != nil {
		t.Fatalf("comment is optional: %v", err)
	}

	tests := []struct {
		name string
		in   FeedbackInput
		want error
	}{
		{"bad rating", FeedbackInput{UserID: userID, ExplanationID: saved.ID, Rating: "meh"}, domain.ErrInvalidFeedbackInput},
		{"blank comment", FeedbackInput{UserID: userID, ExplanationID: saved.ID, Rating: "like", Comment: ptr("  ")}, domain.ErrInvalidFeedbackInput},
		{"comment over 500", FeedbackInput{UserID: userID, ExplanationID: saved.ID, Rating: "like", Comment: ptr(strings.Repeat("x", 501))}, domain.ErrInvalidFeedbackInput},
		{"another user", FeedbackInput{UserID: uuid.New(), ExplanationID: saved.ID, Rating: "like"}, domain.ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.UpsertFeedback(ctx, tt.in); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestListHighlightExplanationsOwnership(t *testing.T) {
	repo := &fakeRepo{}
	generator := &fakeGenerator{}
	saved := savedExplanation(t, repo, generator)
	svc := NewAnalogyService(repo, generator)
	ctx := context.Background()

	if _, err := svc.ListHighlightExplanations(ctx, uuid.New(), saved.HighlightID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if _, err := svc.ListHighlightExplanations(ctx, userID, uuid.New()); !errors.Is(err, domain.ErrHighlightNotFound) {
		t.Fatalf("err = %v, want ErrHighlightNotFound", err)
	}
}
