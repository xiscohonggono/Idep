package handler

import (
	"api/internal/domain"
	"api/internal/service"
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// CallerHeader identifies the caller until JWT middleware lands.
// shortcut: X-User-ID is client-asserted and spoofable, replace with Stage 1 Supabase JWT verification before any public deployment
type CallerHeader struct {
	UserID string `header:"X-User-ID" doc:"Caller's user UUID"`
}

func (c CallerHeader) id() (uuid.UUID, error) {
	id, err := uuid.Parse(c.UserID)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, huma.Error401Unauthorized("missing or invalid X-User-ID")
	}
	return id, nil
}

type CreateExplanationInput struct {
	CallerHeader
	ChapterID uuid.UUID `path:"chapter_id" format:"uuid" doc:"Chapter UUID"`
	Body      struct {
		VersionID    uuid.UUID `json:"version_id" format:"uuid"`
		SelectedText string    `json:"selected_text"`
		StartOffset  int       `json:"start_offset"`
		EndOffset    int       `json:"end_offset"`
		JobFieldCode *string   `json:"job_field_code,omitempty"`
	}
}

type CreateExplanationOutput struct {
	Status int
	Body   domain.AIExplanation
}

type ListExplanationsInput struct {
	CallerHeader
	HighlightID uuid.UUID `path:"highlight_id" format:"uuid" doc:"Highlight UUID"`
}

type ListExplanationsOutput struct {
	Body struct {
		Explanations []domain.AIExplanation `json:"explanations" nullable:"false"`
	}
}

type ElaborateInput struct {
	CallerHeader
	ExplanationID uuid.UUID `path:"explanation_id" format:"uuid" doc:"Explanation UUID"`
	Body          struct {
		TargetSection string  `json:"target_section" enum:"step,term,limit,general"`
		TargetIndex   *int    `json:"target_index,omitempty" minimum:"0"`
		UserPrompt    *string `json:"user_prompt,omitempty" maxLength:"300"`
	}
}

type ElaborateOutput struct {
	Body domain.AIElaboration
}

type FeedbackInput struct {
	CallerHeader
	ExplanationID uuid.UUID `path:"explanation_id" format:"uuid" doc:"Explanation UUID"`
	Body          struct {
		Rating  string  `json:"rating" enum:"like,dislike"`
		Comment *string `json:"comment,omitempty" maxLength:"500"`
	}
}

type FeedbackOutput struct {
	Body domain.ExplanationFeedback
}

// analogyError maps domain errors to typed responses. Anything unrecognized
// fails closed as 503.
func analogyError(operation string, err error) error {
	var status huma.StatusError
	if errors.As(err, &status) {
		return err
	}

	unprocessable := []error{
		domain.ErrSelectionTooLong, domain.ErrSelectionNotInChapter, domain.ErrInvalidJobField,
		domain.ErrInvalidElaborationInput, domain.ErrInvalidFeedbackInput,
	}
	for _, target := range unprocessable {
		if errors.Is(err, target) {
			return huma.Error422UnprocessableEntity(target.Error())
		}
	}

	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		return huma.Error401Unauthorized("missing or invalid X-User-ID")
	case errors.Is(err, domain.ErrForbidden):
		return huma.Error403Forbidden("this belongs to another user")
	case errors.Is(err, domain.ErrChapterNotFound):
		return huma.Error404NotFound("chapter not found")
	case errors.Is(err, domain.ErrHighlightNotFound):
		return huma.Error404NotFound("highlight not found")
	case errors.Is(err, domain.ErrExplanationNotFound):
		return huma.Error404NotFound("explanation not found")
	case errors.Is(err, domain.ErrAIRateLimitExceeded):
		return huma.Error429TooManyRequests("daily AI limit reached")
	case errors.Is(err, domain.ErrAIUnavailable):
		log.Printf("%s failed: %v", operation, err)
		return huma.Error503ServiceUnavailable("AI service unavailable")
	default:
		log.Printf("%s failed: %v", operation, err)
		return huma.Error503ServiceUnavailable("database unavailable")
	}
}

func RegisterAnalogyRoutes(api huma.API, svc *service.AnalogyService) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-chapter-explanation",
		Method:        http.MethodPost,
		Path:          "/api/v1/chapters/{chapter_id}/explanations",
		Summary:       "Explain a highlighted passage through work experience",
		Tags:          []string{"analogies"},
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"200": {
				Description: "A saved explanation was returned without using AI quota",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: &huma.Schema{Ref: "#/components/schemas/AIExplanation"}},
				},
			},
		},
		Errors: []int{
			http.StatusUnauthorized, http.StatusNotFound, http.StatusUnprocessableEntity,
			http.StatusTooManyRequests, http.StatusServiceUnavailable,
		},
	}, func(ctx context.Context, input *CreateExplanationInput) (*CreateExplanationOutput, error) {
		userID, err := input.id()
		if err != nil {
			return nil, err
		}
		explanation, created, err := svc.CreateExplanation(ctx, service.CreateExplanationInput{
			UserID:       userID,
			ChapterID:    input.ChapterID,
			VersionID:    input.Body.VersionID,
			SelectedText: input.Body.SelectedText,
			StartOffset:  input.Body.StartOffset,
			EndOffset:    input.Body.EndOffset,
			JobFieldCode: input.Body.JobFieldCode,
		})
		if err != nil {
			return nil, analogyError("create-chapter-explanation", err)
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		return &CreateExplanationOutput{Status: status, Body: *explanation}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-highlight-explanations",
		Method:      http.MethodGet,
		Path:        "/api/v1/highlights/{highlight_id}/explanations",
		Summary:     "List a highlight's explanations across professions",
		Tags:        []string{"analogies"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusServiceUnavailable},
	}, func(ctx context.Context, input *ListExplanationsInput) (*ListExplanationsOutput, error) {
		userID, err := input.id()
		if err != nil {
			return nil, err
		}
		explanations, err := svc.ListHighlightExplanations(ctx, userID, input.HighlightID)
		if err != nil {
			return nil, analogyError("list-highlight-explanations", err)
		}
		output := &ListExplanationsOutput{}
		output.Body.Explanations = explanations
		return output, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "elaborate-explanation",
		Method:      http.MethodPost,
		Path:        "/api/v1/explanations/{explanation_id}/elaborate",
		Summary:     "Ask for more detail on part of an explanation",
		Tags:        []string{"analogies"},
		Errors: []int{
			http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity,
			http.StatusTooManyRequests, http.StatusServiceUnavailable,
		},
	}, func(ctx context.Context, input *ElaborateInput) (*ElaborateOutput, error) {
		userID, err := input.id()
		if err != nil {
			return nil, err
		}
		elaboration, err := svc.Elaborate(ctx, service.ElaborateInput{
			UserID:        userID,
			ExplanationID: input.ExplanationID,
			TargetSection: input.Body.TargetSection,
			TargetIndex:   input.Body.TargetIndex,
			UserPrompt:    input.Body.UserPrompt,
		})
		if err != nil {
			return nil, analogyError("elaborate-explanation", err)
		}
		return &ElaborateOutput{Body: *elaboration}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "upsert-explanation-feedback",
		Method:      http.MethodPut,
		Path:        "/api/v1/explanations/{explanation_id}/feedback",
		Summary:     "Like or dislike an explanation",
		Tags:        []string{"analogies"},
		Errors: []int{
			http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
			http.StatusUnprocessableEntity, http.StatusServiceUnavailable,
		},
	}, func(ctx context.Context, input *FeedbackInput) (*FeedbackOutput, error) {
		userID, err := input.id()
		if err != nil {
			return nil, err
		}
		feedback, err := svc.UpsertFeedback(ctx, service.FeedbackInput{
			UserID:        userID,
			ExplanationID: input.ExplanationID,
			Rating:        input.Body.Rating,
			Comment:       input.Body.Comment,
		})
		if err != nil {
			return nil, analogyError("upsert-explanation-feedback", err)
		}
		return &FeedbackOutput{Body: *feedback}, nil
	})
}
