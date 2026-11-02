-- +goose Up
SET lock_timeout = '3s';
SET statement_timeout = '30s';

-- 5. Guest Session & 3-Free-Chapter Metering (U-13)
CREATE TABLE guest_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint_hash TEXT NOT NULL,
    converted_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Rate-limiting index for daily salted (IP prefix + User-Agent) fingerprint window (ADR-002 §4.3)
CREATE INDEX idx_guest_sessions_fingerprint_created
    ON guest_sessions (fingerprint_hash, created_at DESC);

CREATE TABLE guest_chapter_reads (
    guest_session_id UUID NOT NULL REFERENCES guest_sessions(id) ON DELETE CASCADE,
    chapter_id UUID NOT NULL, -- FK added after chapters table creation
    first_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guest_session_id, chapter_id)
);

-- 8. Four Official GED Subjects (U-03)
CREATE TABLE subjects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL UNIQUE CHECK (slug IN (
        'mathematical-reasoning',
        'reasoning-through-language-arts',
        'science',
        'social-studies'
    )),
    title TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    display_order SMALLINT NOT NULL UNIQUE CHECK (display_order BETWEEN 1 AND 4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed 4 Official GED Subjects (U-03)
INSERT INTO subjects (slug, title, description, display_order) VALUES
    ('mathematical-reasoning', 'Mathematical Reasoning', 'Quantitative problem solving, algebraic reasoning, geometry, and workplace math applications.', 1),
    ('reasoning-through-language-arts', 'Reasoning Through Language Arts', 'Reading comprehension, evidence-based argument analysis, grammar, and extended response writing.', 2),
    ('science', 'Science', 'Life science, physical science, Earth and space systems, scientific method, and data interpretation.', 3),
    ('social-studies', 'Social Studies', 'Civics and government, U.S. history, economics, geography, and primary source evaluation.', 4)
ON CONFLICT (slug) DO NOTHING;

-- 9. Chapters Directory (U-03, E-06)
CREATE TABLE chapters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL,
    title TEXT NOT NULL CHECK (char_length(btrim(title)) BETWEEN 3 AND 200),
    display_order INT NOT NULL CHECK (display_order > 0),
    status TEXT NOT NULL DEFAULT 'pending_initial_publish'
        CHECK (status IN ('pending_initial_publish', 'published', 'archived')),
    current_version_id UUID, -- Circular FK to chapter_versions(id) added via ALTER TABLE
    search_tsv TSVECTOR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (subject_id, slug)
);

CREATE INDEX idx_chapters_subject_order
    ON chapters (subject_id, display_order ASC)
    WHERE status = 'published';

CREATE INDEX idx_chapters_search_tsv
    ON chapters USING GIN (search_tsv)
    WHERE status = 'published';

-- 10. Immutable Chapter Versions (U-03, E-05, C-04, C-06, C-08, E-10)
CREATE TABLE chapter_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_id UUID NOT NULL REFERENCES chapters(id) ON DELETE RESTRICT,
    version_number INT NOT NULL CHECK (version_number > 0),
    title TEXT NOT NULL CHECK (char_length(btrim(title)) BETWEEN 3 AND 200),
    content_markdown TEXT NOT NULL CHECK (char_length(btrim(content_markdown)) > 0),
    toc_json JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(toc_json) = 'array'),
    -- Immutable snapshot of numbered source citations (E-05) for this published/seeded/rolled-back version
    citations_json JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(citations_json) = 'array'),
    edit_summary TEXT NOT NULL CHECK (char_length(btrim(edit_summary)) > 0),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    checker_id UUID REFERENCES users(id) ON DELETE RESTRICT, -- Nullable ONLY for initial baseline curriculum seeds
    source_proposal_id UUID, -- FK to proposals(id)
    rollback_from_version_id UUID REFERENCES chapter_versions(id) ON DELETE RESTRICT,
    rollback_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chapter_id, version_number),
    -- Provenance & Two-Person Integrity (U-03, C-04, C-06, C-08):
    -- 1. Standard proposal publication requires a Checker distinct from Author (author_id <> checker_id)
    -- 2. Rollback requires a Checker and non-empty rollback_reason (even if Checker originally authored the restored target version)
    -- 3. Initial baseline curriculum seed has no proposal, no rollback, and checker_id IS NULL
    CONSTRAINT chk_version_provenance CHECK (
        (source_proposal_id IS NOT NULL AND rollback_from_version_id IS NULL AND checker_id IS NOT NULL AND author_id <> checker_id AND rollback_reason IS NULL)
        OR (rollback_from_version_id IS NOT NULL AND source_proposal_id IS NULL AND checker_id IS NOT NULL AND rollback_reason IS NOT NULL AND char_length(btrim(rollback_reason)) > 0)
        OR (source_proposal_id IS NULL AND rollback_from_version_id IS NULL AND checker_id IS NULL AND rollback_reason IS NULL)
    )
);

ALTER TABLE chapters
    ADD CONSTRAINT fk_chapters_current_version
    FOREIGN KEY (current_version_id) REFERENCES chapter_versions(id) ON DELETE RESTRICT;

-- Editor Contribution History & Admin Checker Review Index (E-10, C-09 keyset pagination)
CREATE INDEX idx_chapter_versions_author_history
    ON chapter_versions (author_id, created_at DESC, id DESC);

ALTER TABLE guest_chapter_reads
    ADD CONSTRAINT fk_guest_reads_chapter
    FOREIGN KEY (chapter_id) REFERENCES chapters(id) ON DELETE CASCADE;

-- 11. Curated Chapter Prerequisites (U-08)
CREATE TABLE chapter_prerequisites (
    chapter_version_id UUID NOT NULL REFERENCES chapter_versions(id) ON DELETE CASCADE,
    prerequisite_chapter_id UUID NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    display_order SMALLINT NOT NULL DEFAULT 0,
    PRIMARY KEY (chapter_version_id, prerequisite_chapter_id)
);

CREATE TRIGGER trg_chapter_versions_immutable
    BEFORE UPDATE OR DELETE ON chapter_versions
    FOR EACH ROW EXECUTE FUNCTION prevent_immutable_table_mutation();

-- Automatically synchronize chapters.search_tsv from active chapter_versions (U-03)
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_chapter_search_tsv()
RETURNS TRIGGER AS $$
DECLARE
    v_title TEXT;
    v_content TEXT;
BEGIN
    IF NEW.current_version_id IS NOT NULL AND (
        TG_OP = 'INSERT'
        OR NEW.current_version_id IS DISTINCT FROM OLD.current_version_id
        OR NEW.title IS DISTINCT FROM OLD.title
    ) THEN
        SELECT title, content_markdown
          INTO v_title, v_content
          FROM chapter_versions
         WHERE id = NEW.current_version_id;

        NEW.search_tsv :=
            setweight(to_tsvector('english', coalesce(v_title, NEW.title, '')), 'A') ||
            setweight(to_tsvector('english', coalesce(v_content, '')), 'B');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_chapters_sync_search_tsv
    BEFORE INSERT OR UPDATE OF current_version_id, title ON chapters
    FOR EACH ROW EXECUTE FUNCTION sync_chapter_search_tsv();

-- +goose Down
SET lock_timeout = '3s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS guest_chapter_reads;
DROP TABLE IF EXISTS guest_sessions;
DROP TABLE IF EXISTS chapter_prerequisites;
DROP TRIGGER IF EXISTS trg_chapters_sync_search_tsv ON chapters;
DROP FUNCTION IF EXISTS sync_chapter_search_tsv();
DROP TRIGGER IF EXISTS trg_chapter_versions_immutable ON chapter_versions;
ALTER TABLE chapters DROP CONSTRAINT IF EXISTS fk_chapters_current_version;
DROP TABLE IF EXISTS chapter_versions;
DROP TABLE IF EXISTS chapters;
DROP TABLE IF EXISTS subjects;
