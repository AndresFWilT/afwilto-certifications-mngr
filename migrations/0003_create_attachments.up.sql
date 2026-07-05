CREATE TABLE attachments (
    id               uuid PRIMARY KEY,
    certification_id uuid NOT NULL REFERENCES certifications(id) ON DELETE CASCADE,
    kind             text NOT NULL,
    file_name        text NOT NULL,
    content_type     text NOT NULL,
    size_bytes       bigint NOT NULL,
    storage_key      text NOT NULL UNIQUE,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT att_kind_enum CHECK (kind IN ('certificate','diploma')),
    CONSTRAINT att_type_enum CHECK (content_type IN ('application/pdf','image/png','image/jpeg')),
    CONSTRAINT att_size_cap  CHECK (size_bytes > 0 AND size_bytes <= 10485760),
    CONSTRAINT att_one_per_kind UNIQUE (certification_id, kind)
);
