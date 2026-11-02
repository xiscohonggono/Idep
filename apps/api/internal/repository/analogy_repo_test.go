package repository

import (
	"api/internal/dbtest"
	"api/internal/domain"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAnalogyRepositoryNilPool(t *testing.T) {
	repo := NewPostgresAnalogyRepository(nil)
	if _, err := repo.ListPublishedCatalog(context.Background()); !errors.Is(err, domain.ErrDatabaseUnavailable) {
		t.Fatalf("err = %v, want ErrDatabaseUnavailable", err)
	}
}

func TestAnalogyRepositoryLive(t *testing.T) {
	dsn := dbtest.New(t)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("open test pool failed")
	}
	t.Cleanup(pool.Close)

	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, statement, args...); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}

	repo := NewPostgresAnalogyRepository(pool)
	author, other := uuid.New(), uuid.New()
	chapter, version, draft := uuid.New(), uuid.New(), uuid.New()
	exec("INSERT INTO users (id, email, display_name) VALUES ($1, 'a@example.com', 'Ada'), ($2, 'b@example.com', 'Bo')", author, other)
	exec(`INSERT INTO chapters (id, subject_id, slug, title, display_order)
	      SELECT $1, s.id, 'cells', 'Cells', 1 FROM subjects s WHERE s.slug = 'science'`, chapter)
	exec(`INSERT INTO chapters (id, subject_id, slug, title, display_order)
	      SELECT $1, s.id, 'draft', 'Draft', 2 FROM subjects s WHERE s.slug = 'science'`, draft)
	exec(`INSERT INTO chapter_versions (id, chapter_id, version_number, title, content_markdown, edit_summary, author_id)
	      VALUES ($1, $2, 1, 'Cells', 'Cells make energy for the body.', 'Seed', $3)`, version, chapter, author)

	if _, err := repo.GetPublishedChapterVersion(ctx, chapter, version); !errors.Is(err, domain.ErrChapterNotFound) {
		t.Fatalf("unpublished chapter: err = %v", err)
	}
	exec("UPDATE chapters SET status = 'published', current_version_id = $1 WHERE id = $2", version, chapter)
	loaded, err := repo.GetPublishedChapterVersion(ctx, chapter, version)
	if err != nil || loaded.ContentMarkdown != "Cells make energy for the body." {
		t.Fatalf("chapter = %+v, err = %v", loaded, err)
	}

	catalog, err := repo.ListPublishedCatalog(ctx)
	if err != nil || len(catalog) != 1 || catalog[0].ID != chapter {
		t.Fatalf("catalog = %+v, err = %v (the draft must be excluded)", catalog, err)
	}

	if exists, err := repo.UserExists(ctx, uuid.New()); err != nil || exists {
		t.Fatalf("unknown user: exists = %v, err = %v", exists, err)
	}
	if exists, err := repo.UserExists(ctx, author); err != nil || !exists {
		t.Fatalf("known user: exists = %v, err = %v", exists, err)
	}

	exec(`INSERT INTO user_work_experiences (user_id, job_field_code, role_title, priority_order)
	      VALUES ($1, 'healthcare', NULL, 1), ($1, 'fnb', 'line cook', 0)`, author)
	experiences, err := repo.ListUserWorkExperiences(ctx, author)
	if err != nil || len(experiences) != 2 || experiences[0].JobFieldCode != "fnb" || *experiences[0].RoleTitle != "line cook" {
		t.Fatalf("experiences = %+v, err = %v", experiences, err)
	}

	if _, err := repo.GetActiveJobField(ctx, "astronaut"); !errors.Is(err, domain.ErrInvalidJobField) {
		t.Fatalf("unknown field: err = %v", err)
	}
	if field, err := repo.GetActiveJobField(ctx, "fnb"); err != nil || field.Name != "Food & Beverage" {
		t.Fatalf("field = %+v, err = %v", field, err)
	}

	selection := domain.StudyHighlight{
		UserID: author, SubjectID: loaded.SubjectID, ChapterID: chapter, ChapterVersionID: version,
		SelectedText: "Cells make energy", ParagraphCount: 1, StartOffset: 0, EndOffset: 17,
	}
	if found, err := repo.FindHighlight(ctx, selection); err != nil || found != nil {
		t.Fatalf("empty lookup: %+v, %v", found, err)
	}
	highlight, err := repo.CreateHighlight(ctx, selection)
	if err != nil {
		t.Fatal(err)
	}
	if found, err := repo.FindHighlight(ctx, selection); err != nil || found == nil || found.ID != highlight.ID {
		t.Fatalf("lookup: %+v, %v", found, err)
	}
	different := selection
	different.SelectedText = "energy for the body"
	if found, err := repo.FindHighlight(ctx, different); err != nil || found != nil {
		t.Fatalf("different text at the same offsets matched: %+v, %v", found, err)
	}

	fnb, linked := "fnb", "cells"
	explanation := domain.AIExplanation{
		HighlightID: highlight.ID, UserID: author, JobFieldCode: &fnb,
		Terms:           []domain.TermDefinition{{Term: "cell", Definition: "unit", LinkedChapterID: &chapter, LinkedChapterSlug: &linked}},
		AnalogySteps:    []domain.AnalogyStep{{StepNumber: 1, WorkElement: "oven", SubjectTerm: "cell", Explanation: "heat"}},
		AnalogyLimits:   []domain.AnalogyLimit{{AnalogyLimit: "ovens", AccurateExplanation: "cells"}},
		ModelIdentifier: "m", PromptVersion: "v1",
	}
	saved, err := repo.CreateExplanation(ctx, explanation)
	if err != nil {
		t.Fatal(err)
	}

	everyday, err := repo.FindExplanation(ctx, highlight.ID, nil)
	if err != nil || everyday != nil {
		t.Fatalf("everyday lookup: %+v, %v", everyday, err)
	}
	cached, err := repo.FindExplanation(ctx, highlight.ID, &fnb)
	if err != nil || cached == nil || cached.ID != saved.ID || cached.AnalogySteps[0].WorkElement != "oven" {
		t.Fatalf("field lookup: %+v, %v", cached, err)
	}

	fallback := explanation
	fallback.JobFieldCode, fallback.IsEverydayFallback = nil, true
	if _, err := repo.CreateExplanation(ctx, fallback); err != nil {
		t.Fatal(err)
	}
	if found, err := repo.FindExplanation(ctx, highlight.ID, nil); err != nil || found == nil || !found.IsEverydayFallback {
		t.Fatalf("everyday lookup: %+v, %v", found, err)
	}
	if listed, err := repo.ListExplanations(ctx, highlight.ID); err != nil || len(listed) != 2 {
		t.Fatalf("listed = %d, err = %v", len(listed), err)
	}

	joined, err := repo.GetExplanation(ctx, saved.ID)
	if err != nil || joined.SelectedText != selection.SelectedText || joined.ChapterID != chapter || joined.EndOffset != 17 || *joined.Terms[0].LinkedChapterID != chapter {
		t.Fatalf("joined = %+v, err = %v", joined, err)
	}
	if _, err := repo.GetExplanation(ctx, uuid.New()); !errors.Is(err, domain.ErrExplanationNotFound) {
		t.Fatalf("missing explanation: err = %v", err)
	}

	index, hint := 0, "Back to the passage."
	if _, err := repo.CreateElaboration(ctx, author, domain.AIElaboration{
		ExplanationID: saved.ID, TargetSection: domain.SectionStep, TargetIndex: &index, HintText: &hint, ModelIdentifier: "m",
	}); err != nil {
		t.Fatal(err)
	}
	elaborations, err := repo.ListElaborations(ctx, saved.ID)
	if err != nil || len(elaborations) != 1 || *elaborations[0].TargetIndex != 0 {
		t.Fatalf("elaborations = %+v, err = %v", elaborations, err)
	}

	if count, err := repo.CountRecentAICalls(ctx, author); err != nil || count != 3 {
		t.Fatalf("count = %d, err = %v, want 2 explanations + 1 elaboration", count, err)
	}
	if count, err := repo.CountRecentAICalls(ctx, other); err != nil || count != 0 {
		t.Fatalf("other user's count = %d, err = %v", count, err)
	}

	if found, err := repo.GetUserFeedback(ctx, saved.ID, author); err != nil || found != nil {
		t.Fatalf("no feedback yet: %+v, %v", found, err)
	}
	comment := "Clicked"
	feedback := domain.ExplanationFeedback{ExplanationID: saved.ID, UserID: author, ChapterID: chapter, JobFieldCode: &fnb, Rating: "like", Comment: &comment}
	first, err := repo.UpsertFeedback(ctx, feedback)
	if err != nil {
		t.Fatal(err)
	}
	exemplars, err := repo.ListLikedExemplars(ctx, chapter, fnb)
	if err != nil || len(exemplars) != 1 || exemplars[0].Steps[0].WorkElement != "oven" || *exemplars[0].Comment != comment {
		t.Fatalf("exemplars = %+v, err = %v", exemplars, err)
	}

	feedback.Rating, comment = "dislike", "Felt wrong"
	second, err := repo.UpsertFeedback(ctx, feedback)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Rating != "dislike" {
		t.Fatalf("upsert created a second row or kept the rating: %+v", second)
	}
	pitfalls, err := repo.ListDislikedPitfalls(ctx, chapter, fnb)
	if err != nil || len(pitfalls) != 1 || pitfalls[0] != "Felt wrong" {
		t.Fatalf("pitfalls = %v, err = %v", pitfalls, err)
	}
	if exemplars, err := repo.ListLikedExemplars(ctx, chapter, fnb); err != nil || len(exemplars) != 0 {
		t.Fatalf("exemplars after dislike = %d, err = %v", len(exemplars), err)
	}
	if found, err := repo.GetUserFeedback(ctx, saved.ID, author); err != nil || found == nil || found.Rating != "dislike" {
		t.Fatalf("feedback = %+v, err = %v", found, err)
	}
}
