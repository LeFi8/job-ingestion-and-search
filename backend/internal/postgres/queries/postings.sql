-- name: UpsertPosting :one
INSERT INTO postings (
  source_id,
  idempotency_key,
  content_hash,
  raw,
  normalized,
  title,
  company,
  city,
  country,
  remote,
  salary_amount,
  salary_currency,
  salary_unit,
  salary_usd_annual,
  currency_inferred,
  fx_rate,
  posted_on,
  status,
  reasons,
  market,
  policy_version,
  decided_at
) VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8, $9, $10,
  $11, $12, $13, $14, $15, $16,
  $17, $18, $19, $20, $21, $22
)
ON CONFLICT (idempotency_key) DO UPDATE
SET
  content_hash      = EXCLUDED.content_hash,
  raw               = EXCLUDED.raw,
  normalized        = EXCLUDED.normalized,
  title             = EXCLUDED.title,
  company           = EXCLUDED.company,
  city              = EXCLUDED.city,
  country           = EXCLUDED.country,
  remote            = EXCLUDED.remote,
  salary_amount     = EXCLUDED.salary_amount,
  salary_currency   = EXCLUDED.salary_currency,
  salary_unit       = EXCLUDED.salary_unit,
  salary_usd_annual = EXCLUDED.salary_usd_annual,
  currency_inferred = EXCLUDED.currency_inferred,
  fx_rate           = EXCLUDED.fx_rate,
  posted_on         = EXCLUDED.posted_on,
  status            = EXCLUDED.status,
  reasons           = EXCLUDED.reasons,
  market            = EXCLUDED.market,
  policy_version    = EXCLUDED.policy_version,
  decided_at        = EXCLUDED.decided_at
WHERE postings.content_hash IS DISTINCT FROM EXCLUDED.content_hash
RETURNING id;

-- name: EnqueueAll :execrows
INSERT INTO outbox (posting_id)
SELECT id FROM postings;

-- name: GetApprovedJob :one
SELECT
  id AS posting_id,
  title,
  company,
  city,
  country,
  remote,
  COALESCE(round(salary_usd_annual), 0)::bigint AS salary_usd_annual,
  currency_inferred,
  posted_on,
  salary_amount,
  salary_currency,
  salary_unit,
  (normalized->>'Description')::text AS description,
  COALESCE(normalized->'Location'->>'Region', '')::text AS region,
  (normalized->>'CompanyType')::text AS company_type
FROM postings
WHERE id = $1 AND status = 'approved';
