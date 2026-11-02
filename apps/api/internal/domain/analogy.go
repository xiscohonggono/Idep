package domain

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// AI pipeline bounds from guardrails §4.
const (
	MaxAICallsPerDay       = 30
	MinSelectionRunes      = 15
	MaxSelectionRunes      = 3000
	MaxSelectionParagraphs = 3
	MaxElaborationRunes    = 300
	MaxFeedbackRunes       = 500
	MaxRoleTitleRunes      = 80
)

var (
	ErrUnauthorized            = errors.New("unauthorized")
	ErrForbidden               = errors.New("forbidden")
	ErrChapterNotFound         = errors.New("chapter not found")
	ErrExplanationNotFound     = errors.New("explanation not found")
	ErrHighlightNotFound       = errors.New("highlight not found")
	ErrSelectionTooLong        = errors.New("selection must be 15 to 3000 characters across 1 to 3 paragraphs")
	ErrSelectionNotInChapter   = errors.New("selection does not match the chapter text")
	ErrInvalidJobField         = errors.New("unknown job field")
	ErrInvalidElaborationInput = errors.New("invalid elaboration request")
	ErrInvalidFeedbackInput    = errors.New("invalid feedback")
	ErrAIRateLimitExceeded     = errors.New("daily AI limit reached")
	ErrAIUnavailable           = errors.New("AI service unavailable")
)

// Elaboration target sections.
const (
	SectionStep    = "step"
	SectionTerm    = "term"
	SectionLimit   = "limit"
	SectionGeneral = "general"
)

type TermDefinition struct {
	Term               string     `json:"term"`
	Definition         string     `json:"definition"`
	LinkedChapterID    *uuid.UUID `json:"linked_chapter_id,omitempty" format:"uuid"`
	LinkedChapterSlug  *string    `json:"linked_chapter_slug,omitempty"`
	LinkedChapterTitle *string    `json:"linked_chapter_title,omitempty"`
}

type AnalogyStep struct {
	StepNumber  int    `json:"step_number"`
	WorkElement string `json:"work_element"`
	SubjectTerm string `json:"subject_term"`
	Explanation string `json:"explanation"`
}

type AnalogyLimit struct {
	AnalogyLimit        string `json:"analogy_limit"`
	AccurateExplanation string `json:"accurate_explanation"`
}

type PrerequisiteLink struct {
	ChapterID uuid.UUID `json:"chapter_id" format:"uuid"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Reason    string    `json:"reason"`
}

type AIElaboration struct {
	ID                  uuid.UUID `json:"id" format:"uuid"`
	ExplanationID       uuid.UUID `json:"explanation_id" format:"uuid"`
	TargetSection       string    `json:"target_section" enum:"step,term,limit,general"`
	TargetIndex         *int      `json:"target_index,omitempty"`
	UserPrompt          *string   `json:"user_prompt,omitempty"`
	IsOnTopic           bool      `json:"is_on_topic"`
	HintText            *string   `json:"hint_text,omitempty"`
	ElaborationMarkdown *string   `json:"elaboration_markdown,omitempty"`
	WorkExample         *string   `json:"work_example,omitempty"`
	ModelIdentifier     string    `json:"model_identifier"`
	CreatedAt           time.Time `json:"created_at"`
}

type ExplanationFeedback struct {
	ID            uuid.UUID `json:"id" format:"uuid"`
	ExplanationID uuid.UUID `json:"explanation_id" format:"uuid"`
	UserID        uuid.UUID `json:"user_id" format:"uuid"`
	ChapterID     uuid.UUID `json:"chapter_id" format:"uuid"`
	JobFieldCode  *string   `json:"job_field_code,omitempty"`
	Rating        string    `json:"rating" enum:"like,dislike"`
	Comment       *string   `json:"comment,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AIExplanation carries the highlight's chapter and offsets so a client can
// request a comparison for another profession on the same selection.
type AIExplanation struct {
	ID                 uuid.UUID            `json:"id" format:"uuid"`
	HighlightID        uuid.UUID            `json:"highlight_id" format:"uuid"`
	UserID             uuid.UUID            `json:"user_id" format:"uuid"`
	ChapterID          uuid.UUID            `json:"chapter_id" format:"uuid"`
	ChapterVersionID   uuid.UUID            `json:"chapter_version_id" format:"uuid"`
	StartOffset        int                  `json:"start_offset"`
	EndOffset          int                  `json:"end_offset"`
	JobFieldCode       *string              `json:"job_field_code,omitempty"`
	RoleTitleSnapshot  *string              `json:"role_title_snapshot,omitempty"`
	IsEverydayFallback bool                 `json:"is_everyday_fallback"`
	SelectedText       string               `json:"selected_text"`
	Terms              []TermDefinition     `json:"terms" nullable:"false"`
	AnalogySteps       []AnalogyStep        `json:"analogy_steps" nullable:"false"`
	AnalogyLimits      []AnalogyLimit       `json:"analogy_limits" nullable:"false"`
	Prerequisites      []PrerequisiteLink   `json:"prerequisites" nullable:"false"`
	Elaborations       []AIElaboration      `json:"elaborations" nullable:"false"`
	UserFeedback       *ExplanationFeedback `json:"user_feedback,omitempty"`
	ModelIdentifier    string               `json:"model_identifier"`
	PromptVersion      string               `json:"prompt_version"`
	CreatedAt          time.Time            `json:"created_at"`
}

// StudyHighlight is a learner's stored selection in one chapter version.
type StudyHighlight struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	SubjectID        uuid.UUID
	ChapterID        uuid.UUID
	ChapterVersionID uuid.UUID
	SelectedText     string
	ParagraphCount   int
	StartOffset      int
	EndOffset        int
}

// ChapterVersion is the published text a selection is checked against.
type ChapterVersion struct {
	ChapterID       uuid.UUID
	SubjectID       uuid.UUID
	Title           string
	VersionID       uuid.UUID
	ContentMarkdown string
}

type WorkExperience struct {
	JobFieldCode        string
	RoleTitle           *string
	JobFieldName        string
	JobFieldDescription string
}

type JobField struct {
	Code        string
	Name        string
	Description string
}

type CatalogChapter struct {
	ID    uuid.UUID
	Slug  string
	Title string
}

// LikedExemplar is another learner's liked explanation, reduced to prompt text.
type LikedExemplar struct {
	Steps   []AnalogyStep
	Limits  []AnalogyLimit
	Comment *string
}

// Profession is one generation candidate with its flywheel conditioning. All
// role titles, exemplars, and pitfalls are sanitized before reaching a generator.
type Profession struct {
	JobFieldCode     string
	Name             string
	Description      string
	RoleTitle        *string
	LikedExemplars   []LikedExemplar
	DislikedPitfalls []string
}

// GenerationRequest has no professions when the everyday fallback applies.
type GenerationRequest struct {
	ChapterTitle string
	SelectedText string
	Professions  []Profession
	Catalog      []CatalogChapter
}

type GeneratedExplanation struct {
	SelectedJobFieldCode *string
	Terms                []TermDefinition
	AnalogySteps         []AnalogyStep
	AnalogyLimits        []AnalogyLimit
	Prerequisites        []PrerequisiteLink
	ModelIdentifier      string
	PromptVersion        string
}

// Complete reports whether the explanation has the steps and the populated
// "where this comparison falls short" items required by U-06 and U-07.
func (g *GeneratedExplanation) Complete() bool {
	if len(g.AnalogySteps) == 0 || len(g.AnalogyLimits) == 0 {
		return false
	}
	for _, step := range g.AnalogySteps {
		if isBlank(step.WorkElement) || isBlank(step.SubjectTerm) || isBlank(step.Explanation) {
			return false
		}
	}
	for _, limit := range g.AnalogyLimits {
		if isBlank(limit.AnalogyLimit) || isBlank(limit.AccurateExplanation) {
			return false
		}
	}
	return true
}

type ElaborationRequest struct {
	SelectedText  string
	Profession    string
	TargetSection string
	TargetContext string
	UserPrompt    *string // Sanitized
}

type GeneratedElaboration struct {
	IsOnTopic           bool
	HintText            *string
	ElaborationMarkdown *string
	WorkExample         *string
	ModelIdentifier     string
}

// Complete reports whether the elaboration has the content its topic verdict
// requires, matching chk_elaboration_content_or_hint.
func (g *GeneratedElaboration) Complete() bool {
	if g.IsOnTopic {
		return g.ElaborationMarkdown != nil && !isBlank(*g.ElaborationMarkdown)
	}
	return g.HintText != nil && !isBlank(*g.HintText)
}

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

// SanitizeUntrusted strips angle brackets and control characters and caps the
// length, so learner text cannot close an untrusted-data delimiter in a prompt.
func SanitizeUntrusted(s string, maxRunes int) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '<' || r == '>' || unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	runes := []rune(strings.TrimSpace(cleaned))
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes)
}

type AnalogyRepository interface {
	UserExists(ctx context.Context, userID uuid.UUID) (bool, error)
	GetPublishedChapterVersion(ctx context.Context, chapterID, versionID uuid.UUID) (*ChapterVersion, error)
	ListUserWorkExperiences(ctx context.Context, userID uuid.UUID) ([]WorkExperience, error)
	GetActiveJobField(ctx context.Context, code string) (*JobField, error)
	ListPublishedCatalog(ctx context.Context) ([]CatalogChapter, error)
	CountRecentAICalls(ctx context.Context, userID uuid.UUID) (int64, error)
	FindHighlight(ctx context.Context, h StudyHighlight) (*StudyHighlight, error)
	CreateHighlight(ctx context.Context, h StudyHighlight) (*StudyHighlight, error)
	GetHighlight(ctx context.Context, id uuid.UUID) (*StudyHighlight, error)
	// FindExplanation returns nil, nil when no explanation exists yet. A nil
	// code selects the everyday fallback.
	FindExplanation(ctx context.Context, highlightID uuid.UUID, jobFieldCode *string) (*AIExplanation, error)
	CreateExplanation(ctx context.Context, e AIExplanation) (*AIExplanation, error)
	GetExplanation(ctx context.Context, id uuid.UUID) (*AIExplanation, error)
	ListExplanations(ctx context.Context, highlightID uuid.UUID) ([]AIExplanation, error)
	ListLikedExemplars(ctx context.Context, chapterID uuid.UUID, jobFieldCode string) ([]LikedExemplar, error)
	ListDislikedPitfalls(ctx context.Context, chapterID uuid.UUID, jobFieldCode string) ([]string, error)
	CreateElaboration(ctx context.Context, userID uuid.UUID, e AIElaboration) (*AIElaboration, error)
	ListElaborations(ctx context.Context, explanationID uuid.UUID) ([]AIElaboration, error)
	UpsertFeedback(ctx context.Context, f ExplanationFeedback) (*ExplanationFeedback, error)
	// GetUserFeedback returns nil, nil when the user has left no feedback.
	GetUserFeedback(ctx context.Context, explanationID, userID uuid.UUID) (*ExplanationFeedback, error)
}

type AnalogyGenerator interface {
	GenerateExplanation(ctx context.Context, req GenerationRequest) (*GeneratedExplanation, error)
	GenerateElaboration(ctx context.Context, req ElaborationRequest) (*GeneratedElaboration, error)
}
