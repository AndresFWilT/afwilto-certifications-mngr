CREATE TABLE certifications (
    id                 uuid PRIMARY KEY,
    category_id        uuid NOT NULL REFERENCES categories(id),
    name               text NOT NULL,
    goal               text,
    status             text NOT NULL DEFAULT 'planned',
    study_start_date   date,
    expected_exam_date date,
    achieved_date      date,
    credly_url         text,
    official_url       text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    CONSTRAINT cert_name_len    CHECK (char_length(name) BETWEEN 1 AND 200),
    CONSTRAINT cert_goal_len    CHECK (goal IS NULL OR char_length(goal) <= 2000),
    CONSTRAINT cert_status_enum CHECK (status IN ('planned','in_progress','achieved')),
    CONSTRAINT cert_dates_order CHECK (
        study_start_date IS NULL OR expected_exam_date IS NULL
        OR expected_exam_date >= study_start_date)
);

-- uniqueness among the living only (soft delete must free the name)
CREATE UNIQUE INDEX certifications_name_alive_ux
    ON certifications (lower(name)) WHERE deleted_at IS NULL;

CREATE INDEX certifications_alive_status_ix
    ON certifications (status) WHERE deleted_at IS NULL;
CREATE INDEX certifications_alive_category_ix
    ON certifications (category_id) WHERE deleted_at IS NULL;
CREATE INDEX certifications_alive_dates_ix
    ON certifications (study_start_date, expected_exam_date) WHERE deleted_at IS NULL;
CREATE INDEX certifications_alive_created_ix
    ON certifications (created_at DESC) WHERE deleted_at IS NULL;
