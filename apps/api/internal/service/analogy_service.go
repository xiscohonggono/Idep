package service

import (
	"api/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// paragraphBreak also matches blank lines holding spaces or carriage returns,
// so those cannot pass a long selection off as one paragraph.
var paragraphBreak = regexp.MustCompile(`\n[ \t\r]*\n`)

type AnalogyService struct {
	repo      domain.AnalogyRepository
	generator domain.AnalogyGenerator
}

func NewAnalogyService(repo domain.AnalogyRepository, generator domain.AnalogyGenerator) *AnalogyService {
	return &AnalogyService{repo: repo, generator: generator}
}

type CreateExplanationInput struct {
	UserID       uuid.UUID
	ChapterID    uuid.UUID
	VersionID    uuid.UUID
	SelectedText string
	StartOffset  int
	EndOffset    int
	JobFieldCode *string
}

type ElaborateInput struct {
	UserID        uuid.UUID
	ExplanationID uuid.UUID
	TargetSection string
	TargetIndex   *int
	UserPrompt    *string
}

type FeedbackInput struct {
	UserID        uuid.UUID
	ExplanationID uuid.UUID
	Rating        string
	Comment       *string
}

func paragraphCount(text string) int {
	count := 0
	for _, paragraph := range paragraphBreak.Split(text, -1) {
		if strings.TrimSpace(paragraph) != "" {
			count++
		}
	}
	return count
}

// CreateExplanation returns a saved explanation (created = false) without
// spending quota, or generates and stores a new one (created = true).
func (s *AnalogyService) CreateExplanation(ctx context.Context, in CreateExplanationInput) (*domain.AIExplanation, bool, error) {
	selected := strings.TrimSpace(in.SelectedText)
	runes := len([]rune(selected))
	paragraphs := paragraphCount(selected)
	if runes < domain.MinSelectionRunes || runes > domain.MaxSelectionRunes ||
		paragraphs < 1 || paragraphs > domain.MaxSelectionParagraphs {
		return nil, false, domain.ErrSelectionTooLong
	}

	// An unknown ID must not reach the LLM: it would have no rows to count, so
	// the quota could never stop it.
	exists, err := s.repo.UserExists(ctx, in.UserID)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, domain.ErrUnauthorized
	}

	chapter, err := s.repo.GetPublishedChapterVersion(ctx, in.ChapterID, in.VersionID)
	if err != nil {
		return nil, false, err
	}
	// Byte length bounds any client offset unit and keeps offsets inside INT.
	if in.StartOffset < 0 || in.EndOffset <= in.StartOffset || in.EndOffset > len(chapter.ContentMarkdown) ||
		!strings.Contains(chapter.ContentMarkdown, selected) {
		return nil, false, domain.ErrSelectionNotInChapter
	}

	experiences, err := s.repo.ListUserWorkExperiences(ctx, in.UserID)
	if err != nil {
		return nil, false, err
	}

	candidates, err := s.candidates(ctx, in.JobFieldCode, experiences)
	if err != nil {
		return nil, false, err
	}

	wanted := domain.StudyHighlight{
		UserID:           in.UserID,
		SubjectID:        chapter.SubjectID,
		ChapterID:        chapter.ChapterID,
		ChapterVersionID: chapter.VersionID,
		SelectedText:     selected,
		ParagraphCount:   paragraphs,
		StartOffset:      in.StartOffset,
		EndOffset:        in.EndOffset,
	}
	highlight, err := s.repo.FindHighlight(ctx, wanted)
	if err != nil {
		return nil, false, err
	}

	catalog, err := s.repo.ListPublishedCatalog(ctx)
	if err != nil {
		return nil, false, err
	}

	if highlight != nil {
		cached, err := s.cachedExplanation(ctx, highlight, in.JobFieldCode, candidates)
		if err != nil {
			return nil, false, err
		}
		if cached != nil {
			if err := s.hydrate(ctx, cached, highlight, catalog); err != nil {
				return nil, false, err
			}
			return cached, false, nil
		}
	}

	if err := s.checkQuota(ctx, in.UserID); err != nil {
		return nil, false, err
	}

	for i := range candidates {
		if err := s.condition(ctx, chapter.ChapterID, &candidates[i]); err != nil {
			return nil, false, err
		}
	}

	generated, err := s.generator.GenerateExplanation(ctx, domain.GenerationRequest{
		ChapterTitle: chapter.Title,
		SelectedText: selected,
		Professions:  sanitizedProfessions(candidates),
		Catalog:      catalog,
	})
	if err != nil {
		return nil, false, err
	}
	if !generated.Complete() {
		return nil, false, domain.ErrAIUnavailable
	}

	if highlight == nil {
		if highlight, err = s.repo.CreateHighlight(ctx, wanted); err != nil {
			return nil, false, err
		}
	}

	explanation := domain.AIExplanation{
		HighlightID:     highlight.ID,
		UserID:          in.UserID,
		Terms:           filterTerms(generated.Terms, catalog),
		AnalogySteps:    generated.AnalogySteps,
		AnalogyLimits:   generated.AnalogyLimits,
		Prerequisites:   filterPrerequisites(generated.Prerequisites, catalog, chapter.ChapterID),
		ModelIdentifier: generated.ModelIdentifier,
		PromptVersion:   generated.PromptVersion,
	}
	if len(candidates) == 0 {
		explanation.IsEverydayFallback = true
	} else {
		chosen := pickProfession(candidates, generated.SelectedJobFieldCode)
		explanation.JobFieldCode = &chosen.JobFieldCode
		explanation.RoleTitleSnapshot = chosen.RoleTitle
	}

	saved, err := s.repo.CreateExplanation(ctx, explanation)
	if err != nil {
		return nil, false, err
	}
	if err := s.hydrate(ctx, saved, highlight, catalog); err != nil {
		return nil, false, err
	}
	return saved, true, nil
}

// candidates resolves who the analogy can be written for: the explicit field,
// the saved professions, or nobody (everyday fallback).
func (s *AnalogyService) candidates(ctx context.Context, code *string, experiences []domain.WorkExperience) ([]domain.Profession, error) {
	if code == nil {
		professions := make([]domain.Profession, 0, len(experiences))
		for _, exp := range experiences {
			professions = append(professions, domain.Profession{
				JobFieldCode: exp.JobFieldCode,
				Name:         exp.JobFieldName,
				Description:  exp.JobFieldDescription,
				RoleTitle:    exp.RoleTitle,
			})
		}
		return professions, nil
	}

	field, err := s.repo.GetActiveJobField(ctx, *code)
	if err != nil {
		return nil, err
	}
	profession := domain.Profession{JobFieldCode: field.Code, Name: field.Name, Description: field.Description}
	for _, exp := range experiences {
		if exp.JobFieldCode == field.Code {
			profession.RoleTitle = exp.RoleTitle
		}
	}
	return []domain.Profession{profession}, nil
}

// cachedExplanation finds a saved explanation. With several saved professions
// and no explicit field the choice is the LLM's, so the highlight's first
// explanation is reused instead of paying to choose again.
func (s *AnalogyService) cachedExplanation(ctx context.Context, highlight *domain.StudyHighlight, code *string, candidates []domain.Profession) (*domain.AIExplanation, error) {
	switch {
	case code != nil:
		return s.repo.FindExplanation(ctx, highlight.ID, code)
	case len(candidates) == 0:
		return s.repo.FindExplanation(ctx, highlight.ID, nil)
	case len(candidates) == 1:
		return s.repo.FindExplanation(ctx, highlight.ID, &candidates[0].JobFieldCode)
	}
	existing, err := s.repo.ListExplanations(ctx, highlight.ID)
	if err != nil || len(existing) == 0 {
		return nil, err
	}
	return &existing[0], nil
}

func (s *AnalogyService) checkQuota(ctx context.Context, userID uuid.UUID) error {
	// shortcut: check-then-insert lets concurrent requests overshoot the quota slightly, serialize per user if abuse appears
	count, err := s.repo.CountRecentAICalls(ctx, userID)
	if err != nil {
		return err
	}
	if count >= domain.MaxAICallsPerDay {
		return domain.ErrAIRateLimitExceeded
	}
	return nil
}

func (s *AnalogyService) condition(ctx context.Context, chapterID uuid.UUID, p *domain.Profession) error {
	var err error
	if p.LikedExemplars, err = s.repo.ListLikedExemplars(ctx, chapterID, p.JobFieldCode); err != nil {
		return err
	}
	p.DislikedPitfalls, err = s.repo.ListDislikedPitfalls(ctx, chapterID, p.JobFieldCode)
	return err
}

// sanitizedProfessions copies the candidates, so the stored role title snapshot
// keeps the learner's text while the prompt gets the sanitized form.
func sanitizedProfessions(candidates []domain.Profession) []domain.Profession {
	professions := make([]domain.Profession, len(candidates))
	for i, p := range candidates {
		if p.RoleTitle != nil {
			title := domain.SanitizeUntrusted(*p.RoleTitle, domain.MaxRoleTitleRunes)
			p.RoleTitle = &title
		}

		exemplars := make([]domain.LikedExemplar, len(p.LikedExemplars))
		for j, exemplar := range p.LikedExemplars {
			exemplars[j] = sanitizedExemplar(exemplar)
		}
		p.LikedExemplars = exemplars

		pitfalls := make([]string, 0, len(p.DislikedPitfalls))
		for _, pitfall := range p.DislikedPitfalls {
			if cleaned := domain.SanitizeUntrusted(pitfall, domain.MaxFeedbackRunes); cleaned != "" {
				pitfalls = append(pitfalls, cleaned)
			}
		}
		p.DislikedPitfalls = pitfalls
		professions[i] = p
	}
	return professions
}

// sanitizedExemplar cleans the stored steps and limits as well as the comment:
// they are model output shaped by another learner's text.
func sanitizedExemplar(e domain.LikedExemplar) domain.LikedExemplar {
	clean := func(s string) string { return domain.SanitizeUntrusted(s, domain.MaxFeedbackRunes) }

	out := domain.LikedExemplar{
		Steps:  make([]domain.AnalogyStep, len(e.Steps)),
		Limits: make([]domain.AnalogyLimit, len(e.Limits)),
	}
	for i, step := range e.Steps {
		out.Steps[i] = domain.AnalogyStep{
			StepNumber:  step.StepNumber,
			WorkElement: clean(step.WorkElement),
			SubjectTerm: clean(step.SubjectTerm),
			Explanation: clean(step.Explanation),
		}
	}
	for i, limit := range e.Limits {
		out.Limits[i] = domain.AnalogyLimit{
			AnalogyLimit:        clean(limit.AnalogyLimit),
			AccurateExplanation: clean(limit.AccurateExplanation),
		}
	}
	if e.Comment != nil {
		if comment := clean(*e.Comment); comment != "" {
			out.Comment = &comment
		}
	}
	return out
}

// pickProfession trusts the LLM's choice only if it names a candidate.
// Candidates are in priority_order, so the first one is the fallback.
func pickProfession(candidates []domain.Profession, selected *string) *domain.Profession {
	if selected != nil {
		for i := range candidates {
			if candidates[i].JobFieldCode == *selected {
				return &candidates[i]
			}
		}
	}
	return &candidates[0]
}

func catalogIndex(catalog []domain.CatalogChapter) map[uuid.UUID]domain.CatalogChapter {
	index := make(map[uuid.UUID]domain.CatalogChapter, len(catalog))
	for _, chapter := range catalog {
		index[chapter.ID] = chapter
	}
	return index
}

// filterTerms keeps every definition but only links to published chapters.
func filterTerms(terms []domain.TermDefinition, catalog []domain.CatalogChapter) []domain.TermDefinition {
	index := catalogIndex(catalog)
	filtered := make([]domain.TermDefinition, 0, len(terms))
	for _, term := range terms {
		term.LinkedChapterSlug, term.LinkedChapterTitle = nil, nil
		if term.LinkedChapterID != nil {
			if chapter, ok := index[*term.LinkedChapterID]; ok {
				term.LinkedChapterSlug, term.LinkedChapterTitle = &chapter.Slug, &chapter.Title
			} else {
				term.LinkedChapterID = nil
			}
		}
		filtered = append(filtered, term)
	}
	return filtered
}

func filterPrerequisites(links []domain.PrerequisiteLink, catalog []domain.CatalogChapter, current uuid.UUID) []domain.PrerequisiteLink {
	index := catalogIndex(catalog)
	filtered := make([]domain.PrerequisiteLink, 0, len(links))
	for _, link := range links {
		chapter, ok := index[link.ChapterID]
		if !ok || link.ChapterID == current {
			continue
		}
		link.Slug, link.Title = chapter.Slug, chapter.Title
		filtered = append(filtered, link)
	}
	return filtered
}

// hydrate fills the response fields that come from other tables. Links are
// filtered again because a saved chapter may have been archived since.
func (s *AnalogyService) hydrate(ctx context.Context, e *domain.AIExplanation, h *domain.StudyHighlight, catalog []domain.CatalogChapter) error {
	e.SelectedText = h.SelectedText
	e.ChapterID = h.ChapterID
	e.ChapterVersionID = h.ChapterVersionID
	e.StartOffset = h.StartOffset
	e.EndOffset = h.EndOffset
	e.Terms = filterTerms(e.Terms, catalog)
	e.Prerequisites = filterPrerequisites(e.Prerequisites, catalog, h.ChapterID)

	elaborations, err := s.repo.ListElaborations(ctx, e.ID)
	if err != nil {
		return err
	}
	e.Elaborations = elaborations
	e.UserFeedback, err = s.repo.GetUserFeedback(ctx, e.ID, e.UserID)
	return err
}

func (s *AnalogyService) ListHighlightExplanations(ctx context.Context, userID, highlightID uuid.UUID) ([]domain.AIExplanation, error) {
	highlight, err := s.repo.GetHighlight(ctx, highlightID)
	if err != nil {
		return nil, err
	}
	if highlight.UserID != userID {
		return nil, domain.ErrForbidden
	}

	explanations, err := s.repo.ListExplanations(ctx, highlightID)
	if err != nil {
		return nil, err
	}
	catalog, err := s.repo.ListPublishedCatalog(ctx)
	if err != nil {
		return nil, err
	}
	for i := range explanations {
		if err := s.hydrate(ctx, &explanations[i], highlight, catalog); err != nil {
			return nil, err
		}
	}
	return explanations, nil
}

// ownedExplanation loads an explanation for its owner, fail closed.
func (s *AnalogyService) ownedExplanation(ctx context.Context, userID, id uuid.UUID) (*domain.AIExplanation, error) {
	explanation, err := s.repo.GetExplanation(ctx, id)
	if err != nil {
		return nil, err
	}
	if explanation.UserID != userID {
		return nil, domain.ErrForbidden
	}
	return explanation, nil
}

// elaborationTarget validates the anchor and returns its text for the prompt.
func elaborationTarget(e *domain.AIExplanation, section string, index *int) (string, error) {
	pick := func(length int) (int, error) {
		if index == nil || *index < 0 || *index >= length {
			return 0, domain.ErrInvalidElaborationInput
		}
		return *index, nil
	}

	var target any
	switch section {
	case domain.SectionGeneral:
		if index != nil {
			return "", domain.ErrInvalidElaborationInput
		}
		return "", nil
	case domain.SectionStep:
		i, err := pick(len(e.AnalogySteps))
		if err != nil {
			return "", err
		}
		target = e.AnalogySteps[i]
	case domain.SectionTerm:
		i, err := pick(len(e.Terms))
		if err != nil {
			return "", err
		}
		target = e.Terms[i]
	case domain.SectionLimit:
		i, err := pick(len(e.AnalogyLimits))
		if err != nil {
			return "", err
		}
		target = e.AnalogyLimits[i]
	default:
		return "", domain.ErrInvalidElaborationInput
	}

	encoded, err := json.Marshal(target)
	if err != nil {
		return "", fmt.Errorf("encode elaboration target: %w", err)
	}
	return string(encoded), nil
}

func (s *AnalogyService) Elaborate(ctx context.Context, in ElaborateInput) (*domain.AIElaboration, error) {
	var userPrompt, promptForLLM *string
	if in.UserPrompt != nil {
		trimmed := strings.TrimSpace(*in.UserPrompt)
		if len([]rune(trimmed)) > domain.MaxElaborationRunes {
			return nil, domain.ErrInvalidElaborationInput
		}
		if trimmed != "" {
			userPrompt = &trimmed
			if cleaned := domain.SanitizeUntrusted(trimmed, domain.MaxElaborationRunes); cleaned != "" {
				promptForLLM = &cleaned
			}
		}
	}

	explanation, err := s.ownedExplanation(ctx, in.UserID, in.ExplanationID)
	if err != nil {
		return nil, err
	}
	target, err := elaborationTarget(explanation, in.TargetSection, in.TargetIndex)
	if err != nil {
		return nil, err
	}
	if err := s.checkQuota(ctx, in.UserID); err != nil {
		return nil, err
	}

	profession := "everyday life"
	if explanation.JobFieldCode != nil {
		profession = *explanation.JobFieldCode
	}
	generated, err := s.generator.GenerateElaboration(ctx, domain.ElaborationRequest{
		SelectedText:  explanation.SelectedText,
		Profession:    profession,
		TargetSection: in.TargetSection,
		TargetContext: target,
		UserPrompt:    promptForLLM,
	})
	if err != nil {
		return nil, err
	}
	if !generated.Complete() {
		return nil, domain.ErrAIUnavailable
	}

	elaboration := domain.AIElaboration{
		ExplanationID:   explanation.ID,
		TargetSection:   in.TargetSection,
		TargetIndex:     in.TargetIndex,
		UserPrompt:      userPrompt,
		IsOnTopic:       generated.IsOnTopic,
		ModelIdentifier: generated.ModelIdentifier,
	}
	if generated.IsOnTopic {
		elaboration.ElaborationMarkdown = generated.ElaborationMarkdown
		elaboration.WorkExample = generated.WorkExample
	} else {
		elaboration.HintText = generated.HintText
	}
	return s.repo.CreateElaboration(ctx, in.UserID, elaboration)
}

func (s *AnalogyService) UpsertFeedback(ctx context.Context, in FeedbackInput) (*domain.ExplanationFeedback, error) {
	if in.Rating != "like" && in.Rating != "dislike" {
		return nil, domain.ErrInvalidFeedbackInput
	}
	var comment *string
	if in.Comment != nil {
		trimmed := strings.TrimSpace(*in.Comment)
		if length := len([]rune(trimmed)); length < 1 || length > domain.MaxFeedbackRunes {
			return nil, domain.ErrInvalidFeedbackInput
		}
		comment = &trimmed
	}

	explanation, err := s.ownedExplanation(ctx, in.UserID, in.ExplanationID)
	if err != nil {
		return nil, err
	}
	return s.repo.UpsertFeedback(ctx, domain.ExplanationFeedback{
		ExplanationID: explanation.ID,
		UserID:        in.UserID,
		ChapterID:     explanation.ChapterID,
		JobFieldCode:  explanation.JobFieldCode,
		Rating:        in.Rating,
		Comment:       comment,
	})
}
