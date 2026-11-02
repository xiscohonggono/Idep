-- name: GetPublishedChapterWithVersion :one
SELECT c.id AS chapter_id, c.subject_id, c.slug, c.title AS chapter_title, c.status,
       cv.id AS version_id, cv.content_markdown
FROM chapters c
JOIN chapter_versions cv ON cv.chapter_id = c.id
WHERE c.id = sqlc.arg(chapter_id) AND cv.id = sqlc.arg(version_id) AND c.status = 'published';

-- name: ListUserWorkExperiences :many
SELECT uwe.job_field_code, uwe.role_title, uwe.priority_order,
       jf.name AS job_field_name, jf.description AS job_field_description
FROM user_work_experiences uwe
JOIN job_fields jf ON jf.code = uwe.job_field_code
WHERE uwe.user_id = $1 AND jf.is_active = TRUE
ORDER BY uwe.priority_order ASC;

-- name: GetActiveJobFieldByCode :one
SELECT code, name, description, example_roles
FROM job_fields
WHERE code = $1 AND is_active = TRUE;

-- name: ListPublishedChaptersCatalog :many
SELECT id, slug, title, subject_id
FROM chapters
WHERE status = 'published'
ORDER BY display_order ASC
LIMIT 200;

-- name: CountUserRecentAICalls :one
SELECT ((SELECT COUNT(*) FROM ai_explanations e WHERE e.user_id = $1 AND e.created_at >= NOW() - INTERVAL '24 hours')
      + (SELECT COUNT(*) FROM ai_elaborations l WHERE l.user_id = $1 AND l.created_at >= NOW() - INTERVAL '24 hours'))::bigint AS total_calls;

-- name: GetStudyHighlightBySelection :one
-- study_highlights has no unique selection index, so the text is matched too
-- and the oldest row wins.
SELECT * FROM study_highlights
WHERE user_id = $1 AND chapter_version_id = $2 AND start_offset = $3 AND end_offset = $4
  AND selected_text = $5
ORDER BY created_at ASC
LIMIT 1;

-- name: CreateStudyHighlight :one
INSERT INTO study_highlights (user_id, subject_id, chapter_id, chapter_version_id, selected_text, paragraph_count, start_offset, end_offset)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetStudyHighlightByID :one
SELECT * FROM study_highlights WHERE id = $1;

-- name: GetExplanationByHighlightAndField :one
SELECT * FROM ai_explanations
WHERE highlight_id = sqlc.arg(highlight_id)
  AND ((is_everyday_fallback = TRUE AND sqlc.arg(is_everyday)::boolean = TRUE)
    OR (is_everyday_fallback = FALSE AND job_field_code = sqlc.narg(job_field_code)))
ORDER BY created_at ASC
LIMIT 1;

-- name: CreateAIExplanation :one
INSERT INTO ai_explanations (highlight_id, user_id, job_field_code, role_title_snapshot, is_everyday_fallback,
                             terms_json, analogy_steps_json, analogy_limits_json, prerequisites_json,
                             model_identifier, prompt_version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetAIExplanationByID :one
SELECT e.id, e.highlight_id, e.user_id, e.job_field_code, e.role_title_snapshot, e.is_everyday_fallback,
       e.terms_json, e.analogy_steps_json, e.analogy_limits_json, e.prerequisites_json,
       e.model_identifier, e.prompt_version, e.created_at,
       h.selected_text, h.chapter_id, h.chapter_version_id, h.subject_id, h.start_offset, h.end_offset
FROM ai_explanations e
JOIN study_highlights h ON h.id = e.highlight_id
WHERE e.id = $1;

-- name: ListExplanationsByHighlight :many
SELECT * FROM ai_explanations
WHERE highlight_id = $1
ORDER BY created_at ASC
LIMIT 20;

-- name: ListLikedProfessionExemplars :many
SELECT e.analogy_steps_json, e.analogy_limits_json, f.comment
FROM ai_explanations e
JOIN ai_explanation_feedback f ON f.explanation_id = e.id
WHERE f.chapter_id = $1 AND f.job_field_code = $2 AND f.rating = 'like'
ORDER BY f.updated_at DESC
LIMIT 3;

-- name: ListDislikedProfessionPitfalls :many
SELECT f.comment
FROM ai_explanation_feedback f
WHERE f.chapter_id = $1 AND f.job_field_code = $2 AND f.rating = 'dislike' AND f.comment IS NOT NULL
ORDER BY f.updated_at DESC
LIMIT 5;

-- name: CreateAIElaboration :one
INSERT INTO ai_elaborations (explanation_id, user_id, target_section, target_index, user_prompt, is_on_topic,
                             hint_text, elaboration_markdown, work_example, model_identifier)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListElaborationsByExplanation :many
SELECT * FROM ai_elaborations
WHERE explanation_id = $1
ORDER BY created_at ASC
LIMIT 50;

-- name: UpsertAIExplanationFeedback :one
INSERT INTO ai_explanation_feedback (explanation_id, user_id, chapter_id, job_field_code, rating, comment)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (explanation_id, user_id) DO UPDATE
SET rating = EXCLUDED.rating, comment = EXCLUDED.comment, updated_at = NOW()
RETURNING *;

-- name: GetUserFeedbackForExplanation :one
SELECT * FROM ai_explanation_feedback
WHERE explanation_id = $1 AND user_id = $2;
