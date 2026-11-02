package repository

import (
	"api/internal/domain"
	"api/internal/repository/db"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAnalogyRepository struct {
	queries *db.Queries
}

func NewPostgresAnalogyRepository(pool *pgxpool.Pool) *PostgresAnalogyRepository {
	r := &PostgresAnalogyRepository{}
	r.SetPool(pool)
	return r
}

// SetPool attaches the database pool after routes are registered, so the
// OpenAPI export never needs a database. Call it before serving requests.
func (r *PostgresAnalogyRepository) SetPool(pool *pgxpool.Pool) {
	if pool != nil {
		r.queries = db.New(pool)
	}
}

// run applies the nil-pool check and the OLTP timeout to one repository call.
func run[T any](ctx context.Context, r *PostgresAnalogyRepository, call func(context.Context, *db.Queries) (T, error)) (T, error) {
	if r.queries == nil {
		var zero T
		return zero, domain.ErrDatabaseUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	return call(ctx, r.queries)
}

// optional turns a missing row into nil, for lookups where absence is normal.
func optional[T any](row T, err error) (*T, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func decodeJSON[T any](raw []byte) ([]T, error) {
	items := []T{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode stored JSON: %w", err)
	}
	return items, nil
}

func encodeJSON[T any](items []T) ([]byte, error) {
	if items == nil {
		items = []T{}
	}
	return json.Marshal(items)
}

func (r *PostgresAnalogyRepository) UserExists(ctx context.Context, userID uuid.UUID) (bool, error) {
	_, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (db.GetUserByIDRow, error) {
		return q.GetUserByID(ctx, userID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}
	return true, nil
}

func (r *PostgresAnalogyRepository) GetPublishedChapterVersion(ctx context.Context, chapterID, versionID uuid.UUID) (*domain.ChapterVersion, error) {
	row, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (db.GetPublishedChapterWithVersionRow, error) {
		return q.GetPublishedChapterWithVersion(ctx, db.GetPublishedChapterWithVersionParams{ChapterID: chapterID, VersionID: versionID})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrChapterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get chapter version: %w", err)
	}
	return &domain.ChapterVersion{
		ChapterID:       row.ChapterID,
		SubjectID:       row.SubjectID,
		Title:           row.ChapterTitle,
		VersionID:       row.VersionID,
		ContentMarkdown: row.ContentMarkdown,
	}, nil
}

func (r *PostgresAnalogyRepository) ListUserWorkExperiences(ctx context.Context, userID uuid.UUID) ([]domain.WorkExperience, error) {
	rows, err := run(ctx, r, func(ctx context.Context, q *db.Queries) ([]db.ListUserWorkExperiencesRow, error) {
		return q.ListUserWorkExperiences(ctx, userID)
	})
	if err != nil {
		return nil, fmt.Errorf("list work experiences: %w", err)
	}
	experiences := make([]domain.WorkExperience, 0, len(rows))
	for _, row := range rows {
		experiences = append(experiences, domain.WorkExperience{
			JobFieldCode:        row.JobFieldCode,
			RoleTitle:           row.RoleTitle,
			JobFieldName:        row.JobFieldName,
			JobFieldDescription: row.JobFieldDescription,
		})
	}
	return experiences, nil
}

func (r *PostgresAnalogyRepository) GetActiveJobField(ctx context.Context, code string) (*domain.JobField, error) {
	row, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (db.GetActiveJobFieldByCodeRow, error) {
		return q.GetActiveJobFieldByCode(ctx, code)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrInvalidJobField
	}
	if err != nil {
		return nil, fmt.Errorf("get job field: %w", err)
	}
	return &domain.JobField{Code: row.Code, Name: row.Name, Description: row.Description}, nil
}

func (r *PostgresAnalogyRepository) ListPublishedCatalog(ctx context.Context) ([]domain.CatalogChapter, error) {
	rows, err := run(ctx, r, func(ctx context.Context, q *db.Queries) ([]db.ListPublishedChaptersCatalogRow, error) {
		return q.ListPublishedChaptersCatalog(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("list catalog: %w", err)
	}
	catalog := make([]domain.CatalogChapter, 0, len(rows))
	for _, row := range rows {
		catalog = append(catalog, domain.CatalogChapter{ID: row.ID, Slug: row.Slug, Title: row.Title})
	}
	return catalog, nil
}

func (r *PostgresAnalogyRepository) CountRecentAICalls(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (int64, error) {
		return q.CountUserRecentAICalls(ctx, userID)
	})
	if err != nil {
		return 0, fmt.Errorf("count AI calls: %w", err)
	}
	return count, nil
}

func highlightFromRow(row db.StudyHighlight) *domain.StudyHighlight {
	return &domain.StudyHighlight{
		ID:               row.ID,
		UserID:           row.UserID,
		SubjectID:        row.SubjectID,
		ChapterID:        row.ChapterID,
		ChapterVersionID: row.ChapterVersionID,
		SelectedText:     row.SelectedText,
		ParagraphCount:   int(row.ParagraphCount),
		StartOffset:      int(row.StartOffset),
		EndOffset:        int(row.EndOffset),
	}
}

func (r *PostgresAnalogyRepository) FindHighlight(ctx context.Context, h domain.StudyHighlight) (*domain.StudyHighlight, error) {
	row, err := optional(run(ctx, r, func(ctx context.Context, q *db.Queries) (db.StudyHighlight, error) {
		return q.GetStudyHighlightBySelection(ctx, db.GetStudyHighlightBySelectionParams{
			UserID:           h.UserID,
			ChapterVersionID: h.ChapterVersionID,
			StartOffset:      int32(h.StartOffset),
			EndOffset:        int32(h.EndOffset),
			SelectedText:     h.SelectedText,
		})
	}))
	if err != nil {
		return nil, fmt.Errorf("find highlight: %w", err)
	}
	if row == nil {
		return nil, nil
	}
	return highlightFromRow(*row), nil
}

func (r *PostgresAnalogyRepository) CreateHighlight(ctx context.Context, h domain.StudyHighlight) (*domain.StudyHighlight, error) {
	row, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (db.StudyHighlight, error) {
		return q.CreateStudyHighlight(ctx, db.CreateStudyHighlightParams{
			UserID:           h.UserID,
			SubjectID:        h.SubjectID,
			ChapterID:        h.ChapterID,
			ChapterVersionID: h.ChapterVersionID,
			SelectedText:     h.SelectedText,
			ParagraphCount:   int16(h.ParagraphCount),
			StartOffset:      int32(h.StartOffset),
			EndOffset:        int32(h.EndOffset),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("create highlight: %w", err)
	}
	return highlightFromRow(row), nil
}

func (r *PostgresAnalogyRepository) GetHighlight(ctx context.Context, id uuid.UUID) (*domain.StudyHighlight, error) {
	row, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (db.StudyHighlight, error) {
		return q.GetStudyHighlightByID(ctx, id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrHighlightNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get highlight: %w", err)
	}
	return highlightFromRow(row), nil
}

func explanationFromRow(row db.AiExplanation) (*domain.AIExplanation, error) {
	terms, err := decodeJSON[domain.TermDefinition](row.TermsJson)
	if err != nil {
		return nil, err
	}
	steps, err := decodeJSON[domain.AnalogyStep](row.AnalogyStepsJson)
	if err != nil {
		return nil, err
	}
	limits, err := decodeJSON[domain.AnalogyLimit](row.AnalogyLimitsJson)
	if err != nil {
		return nil, err
	}
	prerequisites, err := decodeJSON[domain.PrerequisiteLink](row.PrerequisitesJson)
	if err != nil {
		return nil, err
	}
	return &domain.AIExplanation{
		ID:                 row.ID,
		HighlightID:        row.HighlightID,
		UserID:             row.UserID,
		JobFieldCode:       row.JobFieldCode,
		RoleTitleSnapshot:  row.RoleTitleSnapshot,
		IsEverydayFallback: row.IsEverydayFallback,
		Terms:              terms,
		AnalogySteps:       steps,
		AnalogyLimits:      limits,
		Prerequisites:      prerequisites,
		Elaborations:       []domain.AIElaboration{},
		ModelIdentifier:    row.ModelIdentifier,
		PromptVersion:      row.PromptVersion,
		CreatedAt:          row.CreatedAt,
	}, nil
}

func (r *PostgresAnalogyRepository) FindExplanation(ctx context.Context, highlightID uuid.UUID, jobFieldCode *string) (*domain.AIExplanation, error) {
	row, err := optional(run(ctx, r, func(ctx context.Context, q *db.Queries) (db.AiExplanation, error) {
		return q.GetExplanationByHighlightAndField(ctx, db.GetExplanationByHighlightAndFieldParams{
			HighlightID:  highlightID,
			IsEveryday:   jobFieldCode == nil,
			JobFieldCode: jobFieldCode,
		})
	}))
	if err != nil {
		return nil, fmt.Errorf("find explanation: %w", err)
	}
	if row == nil {
		return nil, nil
	}
	return explanationFromRow(*row)
}

func (r *PostgresAnalogyRepository) CreateExplanation(ctx context.Context, e domain.AIExplanation) (*domain.AIExplanation, error) {
	terms, err := encodeJSON(e.Terms)
	if err != nil {
		return nil, err
	}
	steps, err := encodeJSON(e.AnalogySteps)
	if err != nil {
		return nil, err
	}
	limits, err := encodeJSON(e.AnalogyLimits)
	if err != nil {
		return nil, err
	}
	prerequisites, err := encodeJSON(e.Prerequisites)
	if err != nil {
		return nil, err
	}

	row, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (db.AiExplanation, error) {
		return q.CreateAIExplanation(ctx, db.CreateAIExplanationParams{
			HighlightID:        e.HighlightID,
			UserID:             e.UserID,
			JobFieldCode:       e.JobFieldCode,
			RoleTitleSnapshot:  e.RoleTitleSnapshot,
			IsEverydayFallback: e.IsEverydayFallback,
			TermsJson:          terms,
			AnalogyStepsJson:   steps,
			AnalogyLimitsJson:  limits,
			PrerequisitesJson:  prerequisites,
			ModelIdentifier:    e.ModelIdentifier,
			PromptVersion:      e.PromptVersion,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("create explanation: %w", err)
	}
	return explanationFromRow(row)
}

func (r *PostgresAnalogyRepository) GetExplanation(ctx context.Context, id uuid.UUID) (*domain.AIExplanation, error) {
	row, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (db.GetAIExplanationByIDRow, error) {
		return q.GetAIExplanationByID(ctx, id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrExplanationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get explanation: %w", err)
	}

	explanation, err := explanationFromRow(db.AiExplanation{
		ID:                 row.ID,
		HighlightID:        row.HighlightID,
		UserID:             row.UserID,
		JobFieldCode:       row.JobFieldCode,
		RoleTitleSnapshot:  row.RoleTitleSnapshot,
		IsEverydayFallback: row.IsEverydayFallback,
		TermsJson:          row.TermsJson,
		AnalogyStepsJson:   row.AnalogyStepsJson,
		AnalogyLimitsJson:  row.AnalogyLimitsJson,
		PrerequisitesJson:  row.PrerequisitesJson,
		ModelIdentifier:    row.ModelIdentifier,
		PromptVersion:      row.PromptVersion,
		CreatedAt:          row.CreatedAt,
	})
	if err != nil {
		return nil, err
	}
	explanation.SelectedText = row.SelectedText
	explanation.ChapterID = row.ChapterID
	explanation.ChapterVersionID = row.ChapterVersionID
	explanation.StartOffset = int(row.StartOffset)
	explanation.EndOffset = int(row.EndOffset)
	return explanation, nil
}

func (r *PostgresAnalogyRepository) ListExplanations(ctx context.Context, highlightID uuid.UUID) ([]domain.AIExplanation, error) {
	rows, err := run(ctx, r, func(ctx context.Context, q *db.Queries) ([]db.AiExplanation, error) {
		return q.ListExplanationsByHighlight(ctx, highlightID)
	})
	if err != nil {
		return nil, fmt.Errorf("list explanations: %w", err)
	}
	explanations := make([]domain.AIExplanation, 0, len(rows))
	for _, row := range rows {
		explanation, err := explanationFromRow(row)
		if err != nil {
			return nil, err
		}
		explanations = append(explanations, *explanation)
	}
	return explanations, nil
}

func (r *PostgresAnalogyRepository) ListLikedExemplars(ctx context.Context, chapterID uuid.UUID, jobFieldCode string) ([]domain.LikedExemplar, error) {
	rows, err := run(ctx, r, func(ctx context.Context, q *db.Queries) ([]db.ListLikedProfessionExemplarsRow, error) {
		return q.ListLikedProfessionExemplars(ctx, db.ListLikedProfessionExemplarsParams{ChapterID: chapterID, JobFieldCode: &jobFieldCode})
	})
	if err != nil {
		return nil, fmt.Errorf("list liked exemplars: %w", err)
	}
	exemplars := make([]domain.LikedExemplar, 0, len(rows))
	for _, row := range rows {
		steps, err := decodeJSON[domain.AnalogyStep](row.AnalogyStepsJson)
		if err != nil {
			return nil, err
		}
		limits, err := decodeJSON[domain.AnalogyLimit](row.AnalogyLimitsJson)
		if err != nil {
			return nil, err
		}
		exemplars = append(exemplars, domain.LikedExemplar{Steps: steps, Limits: limits, Comment: row.Comment})
	}
	return exemplars, nil
}

func (r *PostgresAnalogyRepository) ListDislikedPitfalls(ctx context.Context, chapterID uuid.UUID, jobFieldCode string) ([]string, error) {
	rows, err := run(ctx, r, func(ctx context.Context, q *db.Queries) ([]*string, error) {
		return q.ListDislikedProfessionPitfalls(ctx, db.ListDislikedProfessionPitfallsParams{ChapterID: chapterID, JobFieldCode: &jobFieldCode})
	})
	if err != nil {
		return nil, fmt.Errorf("list disliked pitfalls: %w", err)
	}
	pitfalls := make([]string, 0, len(rows))
	for _, comment := range rows {
		if comment != nil {
			pitfalls = append(pitfalls, *comment)
		}
	}
	return pitfalls, nil
}

func elaborationFromRow(row db.AiElaboration) domain.AIElaboration {
	var index *int
	if row.TargetIndex != nil {
		value := int(*row.TargetIndex)
		index = &value
	}
	return domain.AIElaboration{
		ID:                  row.ID,
		ExplanationID:       row.ExplanationID,
		TargetSection:       row.TargetSection,
		TargetIndex:         index,
		UserPrompt:          row.UserPrompt,
		IsOnTopic:           row.IsOnTopic,
		HintText:            row.HintText,
		ElaborationMarkdown: row.ElaborationMarkdown,
		WorkExample:         row.WorkExample,
		ModelIdentifier:     row.ModelIdentifier,
		CreatedAt:           row.CreatedAt,
	}
}

func (r *PostgresAnalogyRepository) CreateElaboration(ctx context.Context, userID uuid.UUID, e domain.AIElaboration) (*domain.AIElaboration, error) {
	var index *int16
	if e.TargetIndex != nil {
		value := int16(*e.TargetIndex)
		index = &value
	}
	row, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (db.AiElaboration, error) {
		return q.CreateAIElaboration(ctx, db.CreateAIElaborationParams{
			ExplanationID:       e.ExplanationID,
			UserID:              userID,
			TargetSection:       e.TargetSection,
			TargetIndex:         index,
			UserPrompt:          e.UserPrompt,
			IsOnTopic:           e.IsOnTopic,
			HintText:            e.HintText,
			ElaborationMarkdown: e.ElaborationMarkdown,
			WorkExample:         e.WorkExample,
			ModelIdentifier:     e.ModelIdentifier,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("create elaboration: %w", err)
	}
	created := elaborationFromRow(row)
	return &created, nil
}

func (r *PostgresAnalogyRepository) ListElaborations(ctx context.Context, explanationID uuid.UUID) ([]domain.AIElaboration, error) {
	rows, err := run(ctx, r, func(ctx context.Context, q *db.Queries) ([]db.AiElaboration, error) {
		return q.ListElaborationsByExplanation(ctx, explanationID)
	})
	if err != nil {
		return nil, fmt.Errorf("list elaborations: %w", err)
	}
	elaborations := make([]domain.AIElaboration, 0, len(rows))
	for _, row := range rows {
		elaborations = append(elaborations, elaborationFromRow(row))
	}
	return elaborations, nil
}

func feedbackFromRow(row db.AiExplanationFeedback) *domain.ExplanationFeedback {
	return &domain.ExplanationFeedback{
		ID:            row.ID,
		ExplanationID: row.ExplanationID,
		UserID:        row.UserID,
		ChapterID:     row.ChapterID,
		JobFieldCode:  row.JobFieldCode,
		Rating:        row.Rating,
		Comment:       row.Comment,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func (r *PostgresAnalogyRepository) UpsertFeedback(ctx context.Context, f domain.ExplanationFeedback) (*domain.ExplanationFeedback, error) {
	row, err := run(ctx, r, func(ctx context.Context, q *db.Queries) (db.AiExplanationFeedback, error) {
		return q.UpsertAIExplanationFeedback(ctx, db.UpsertAIExplanationFeedbackParams{
			ExplanationID: f.ExplanationID,
			UserID:        f.UserID,
			ChapterID:     f.ChapterID,
			JobFieldCode:  f.JobFieldCode,
			Rating:        f.Rating,
			Comment:       f.Comment,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("upsert feedback: %w", err)
	}
	return feedbackFromRow(row), nil
}

func (r *PostgresAnalogyRepository) GetUserFeedback(ctx context.Context, explanationID, userID uuid.UUID) (*domain.ExplanationFeedback, error) {
	row, err := optional(run(ctx, r, func(ctx context.Context, q *db.Queries) (db.AiExplanationFeedback, error) {
		return q.GetUserFeedbackForExplanation(ctx, db.GetUserFeedbackForExplanationParams{ExplanationID: explanationID, UserID: userID})
	}))
	if err != nil {
		return nil, fmt.Errorf("get feedback: %w", err)
	}
	if row == nil {
		return nil, nil
	}
	return feedbackFromRow(*row), nil
}
