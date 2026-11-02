-- +goose Up
SET lock_timeout = '3s';
SET statement_timeout = '30s';

-- 3. Canonical Job Field Catalog (U-01, U-02, U-06)
CREATE TABLE job_fields (
    code TEXT PRIMARY KEY CHECK (code ~ '^[a-z0-9_]+$'),
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    example_roles TEXT[] NOT NULL DEFAULT '{}',
    display_order SMALLINT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed 10-Field Starter Catalog (U-01)
INSERT INTO job_fields (code, name, description, example_roles, display_order) VALUES
    ('fnb', 'Food & Beverage', 'Restaurants, cafes, catering, and commercial kitchens', ARRAY['line cook', 'barista', 'server', 'kitchen prep'], 1),
    ('retail', 'Retail & Sales', 'Storefronts, grocery, cashiering, and merchandising', ARRAY['cashier', 'stock associate', 'shift supervisor', 'merchandiser'], 2),
    ('logistics', 'Warehousing, Delivery & Logistics', 'Fulfillment centers, freight, inventory, and last-mile delivery', ARRAY['warehouse associate', 'delivery driver', 'forklift operator', 'dispatcher'], 3),
    ('healthcare', 'Healthcare & Caregiving', 'Clinics, patient care, nursing support, and home health', ARRAY['nursing assistant', 'home health aide', 'medical receptionist', 'orderly'], 4),
    ('construction', 'Construction & Skilled Trades', 'Building trades, electrical, plumbing, and site operations', ARRAY['carpenter', 'electrician apprentice', 'general laborer', 'plumber'], 5),
    ('hospitality', 'Hospitality & Housekeeping', 'Hotels, lodging, facilities upkeep, and guest services', ARRAY['front desk agent', 'housekeeper', 'maintenance technician', 'concierge'], 6),
    ('manufacturing', 'Manufacturing & Assembly', 'Factory production lines, fabrication, and quality control', ARRAY['machine operator', 'assembly line worker', 'quality inspector', 'packager'], 7),
    ('office_admin', 'Office, Clerical & Customer Support', 'Front-office administration, scheduling, billing, and call centers', ARRAY['administrative assistant', 'call center rep', 'data entry clerk', 'receptionist'], 8),
    ('automotive', 'Automotive & Mechanical Repair', 'Vehicle maintenance, diagnostics, parts, and shop operations', ARRAY['auto technician', 'tire & lube tech', 'parts advisor', 'service writer'], 9),
    ('childcare_education', 'Childcare & Community Support', 'Early childhood care, after-school programs, and community services', ARRAY['daycare provider', 'teacher''s aide', 'after-school counselor', 'youth worker'], 10)
ON CONFLICT (code) DO NOTHING;

-- 4. Learner Work-Experience Profile (U-01, U-02)
CREATE TABLE user_work_experiences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_field_code TEXT NOT NULL REFERENCES job_fields(code) ON DELETE RESTRICT,
    role_title TEXT CHECK (role_title IS NULL OR char_length(btrim(role_title)) BETWEEN 1 AND 80),
    priority_order SMALLINT NOT NULL CHECK (priority_order >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, job_field_code)
);

CREATE INDEX idx_user_work_experiences_user_priority
    ON user_work_experiences (user_id, priority_order ASC);

-- +goose Down
SET lock_timeout = '3s';
SET statement_timeout = '30s';

DROP INDEX IF EXISTS idx_user_work_experiences_user_priority;
DROP TABLE IF EXISTS user_work_experiences;
DROP TABLE IF EXISTS job_fields;
