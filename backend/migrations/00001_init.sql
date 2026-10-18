-- squawk-ignore-file require-lock-timeout, require-statement-timeout, ban-drop-table, prefer-bigint-over-int
-- +goose Up
CREATE TABLE postings (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id          text NOT NULL,
  idempotency_key    text NOT NULL UNIQUE,
  content_hash       text NOT NULL,
  raw                bytea NOT NULL,
  normalized         jsonb NOT NULL,
  title              text NOT NULL,
  company            text NOT NULL,
  city               text,
  country            text CHECK (country ~ '^[A-Z]{2}$'),
  remote             boolean NOT NULL,
  salary_amount      numeric(14,2),
  salary_currency    text,
  salary_unit        text CHECK (salary_unit IN ('hourly','annual')),
  salary_usd_annual  numeric(14,2),
  currency_inferred  boolean NOT NULL DEFAULT false,
  fx_rate            numeric(12,6),
  posted_on          date,
  status             text NOT NULL CHECK (status IN ('approved','rejected')),
  reasons            jsonb NOT NULL DEFAULT '[]',
  market             text NOT NULL,
  policy_version     text NOT NULL,
  decided_at         timestamptz NOT NULL,
  ingested_at        timestamptz NOT NULL DEFAULT now(),
  CHECK (status = 'rejected' OR (title <> '' AND salary_usd_annual IS NOT NULL AND (remote OR country IS NOT NULL)))
);

CREATE INDEX idx_postings_source_id ON postings (source_id);

CREATE TABLE outbox (
  id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  posting_id   uuid NOT NULL REFERENCES postings(id) ON DELETE CASCADE,
  created_at   timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz,
  attempts     int NOT NULL DEFAULT 0,
  last_error   text,
  failed_at    timestamptz
);

CREATE INDEX outbox_pending ON outbox (id) WHERE processed_at IS NULL AND failed_at IS NULL;
CREATE INDEX idx_outbox_posting_id ON outbox (posting_id);

CREATE TABLE quarantine (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  source_id     text NOT NULL,
  element_index int NOT NULL,
  raw           bytea NOT NULL,
  raw_hash      bytea GENERATED ALWAYS AS (sha256(raw)) STORED,
  error         text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (source_id, raw_hash)
);

-- +goose Down
DROP TABLE quarantine;
DROP TABLE outbox;
DROP TABLE postings;
