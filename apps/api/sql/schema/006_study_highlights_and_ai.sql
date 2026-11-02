-- +goose Up
SET lock_timeout = '3s';
SET statement_timeout = '30s';

-- 17. Learner Text Highlights & "To Re-Explain" Queue State (U-04, U-09, U-11)
CREATE TABLE study_highlights (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject_id UUID NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    chapter_id UUID NOT NULL REFERENCES chapters(id) ON DELETE RESTRICT,
    chapter_version_id UUID NOT NULL REFERENCES chapter_versions(id) ON DELETE RESTRICT,
    selected_text TEXT NOT NULL CHECK (char_length(btrim(selected_text)) BETWEEN 15 AND 3000),
    paragraph_count SMALLINT NOT NULL CHECK (paragraph_count BETWEEN 1 AND 3),
    start_offset INT NOT NULL CHECK (start_offset >= 0),
    end_offset INT NOT NULL CHECK (end_offset > start_offset),
    reexplain_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (reexplain_status IN ('pending', 'queued', 'completed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Study History grouped by Subject, newest-first (U-11) & "To re-explain" queue (U-09)
CREATE INDEX idx_study_highlights_user_subject_history
    ON study_highlights (user_id, subject_id, created_at DESC, id DESC);

CREATE INDEX idx_study_highlights_reexplain_queue
    ON study_highlights (user_id, created_at DESC, id DESC)
    WHERE reexplain_status = 'queued';

-- 18. Grounded AI Explanations (U-05, U-06, U-07, U-08)
-- Immutable snapshot: updating user_work_experiences (U-02) never mutates existing rows
CREATE TABLE ai_explanations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    highlight_id UUID NOT NULL REFERENCES study_highlights(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_field_code TEXT REFERENCES job_fields(code) ON DELETE SET NULL,
    role_title_snapshot TEXT,
    is_everyday_fallback BOOLEAN NOT NULL DEFAULT FALSE,
    -- U-05: [{ "term": "chlorophyll", "definition": "...", "linked_chapter_id": "..." }]
    terms_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- U-06: [{ "step_number": 1, "work_element": "...", "subject_term": "...", "explanation": "..." }]
    analogy_steps_json JSONB NOT NULL,
    -- U-07: [{ "analogy_limit": "...", "accurate_explanation": "..." }] (Must have >= 1 element)
    analogy_limits_json JSONB NOT NULL CHECK (jsonb_typeof(analogy_limits_json) = 'array' AND jsonb_array_length(analogy_limits_json) >= 1),
    -- U-08: [{ "chapter_id": "...", "slug": "...", "title": "...", "reason": "..." }]
    prerequisites_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    model_identifier TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_work_field_or_fallback CHECK (
        (is_everyday_fallback = TRUE AND job_field_code IS NULL)
        OR (is_everyday_fallback = FALSE AND job_field_code IS NOT NULL)
    )
);

CREATE INDEX idx_ai_explanations_highlight
    ON ai_explanations (highlight_id, created_at DESC);

-- 18b. Passage-Anchored AI Elaborations (Step / Term / Limit / General Deep-Dives)
CREATE TABLE ai_elaborations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    explanation_id UUID NOT NULL REFERENCES ai_explanations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_section TEXT NOT NULL CHECK (target_section IN ('step', 'term', 'limit', 'general')),
    target_index SMALLINT CHECK (target_index IS NULL OR target_index >= 0),
    user_prompt TEXT CHECK (user_prompt IS NULL OR char_length(btrim(user_prompt)) BETWEEN 1 AND 300),
    is_on_topic BOOLEAN NOT NULL,
    hint_text TEXT,
    elaboration_markdown TEXT,
    work_example TEXT,
    model_identifier TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_elaboration_target_index CHECK (
        (target_section = 'general' AND target_index IS NULL)
        OR (target_section IN ('step', 'term', 'limit') AND target_index IS NOT NULL)
    ),
    CONSTRAINT chk_elaboration_content_or_hint CHECK (
        (is_on_topic = TRUE AND elaboration_markdown IS NOT NULL AND char_length(btrim(elaboration_markdown)) > 0)
        OR (is_on_topic = FALSE AND hint_text IS NOT NULL AND char_length(btrim(hint_text)) > 0)
    )
);

CREATE INDEX idx_ai_elaborations_explanation
    ON ai_elaborations (explanation_id, created_at ASC);

CREATE INDEX idx_ai_elaborations_user_quota
    ON ai_elaborations (user_id, created_at DESC);

CREATE INDEX idx_ai_explanations_user_quota
    ON ai_explanations (user_id, created_at DESC);

-- 18c. Learner Like / Dislike & Comment Feedback on AI Explanations (Profession Flywheel)
CREATE TABLE ai_explanation_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    explanation_id UUID NOT NULL REFERENCES ai_explanations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chapter_id UUID NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    job_field_code TEXT REFERENCES job_fields(code) ON DELETE SET NULL,
    rating TEXT NOT NULL CHECK (rating IN ('like', 'dislike')),
    comment TEXT CHECK (comment IS NULL OR char_length(btrim(comment)) BETWEEN 1 AND 500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (explanation_id, user_id)
);

CREATE INDEX idx_ai_explanation_feedback_profession_flywheel
    ON ai_explanation_feedback (chapter_id, job_field_code, rating, updated_at DESC)
    WHERE job_field_code IS NOT NULL;

-- 19. Learner Re-Explanation Attempts & Iterative AI Feedback (U-09, U-10)
CREATE TABLE reexplanation_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    highlight_id UUID NOT NULL REFERENCES study_highlights(id) ON DELETE CASCADE,
    explanation_id UUID NOT NULL REFERENCES ai_explanations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    attempt_number SMALLINT NOT NULL CHECK (attempt_number > 0),
    learner_response TEXT NOT NULL,
    -- U-10: Off-topic or empty responses set is_graded = FALSE and populate hint_text only
    is_graded BOOLEAN NOT NULL,
    hint_text TEXT,
    what_was_right_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    inaccuracies_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    missing_details_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    improvement_summary TEXT,
    model_identifier TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (highlight_id, attempt_number),
    CONSTRAINT chk_ungraded_hint CHECK (
        is_graded = TRUE OR (hint_text IS NOT NULL AND char_length(btrim(hint_text)) > 0)
    )
);

-- +goose Down
SET lock_timeout = '3s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS reexplanation_attempts;
DROP TABLE IF EXISTS ai_explanation_feedback;
DROP TABLE IF EXISTS ai_elaborations;
DROP TABLE IF EXISTS ai_explanations;
DROP TABLE IF EXISTS study_highlights;
