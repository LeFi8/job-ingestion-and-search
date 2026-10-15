-- name: InsertOutbox :exec
INSERT INTO outbox (posting_id)
VALUES ($1);

-- name: ClaimOutbox :many
SELECT
  o.id AS outbox_id,
  o.posting_id,
  p.status,
  p.title,
  p.company,
  p.city,
  p.country,
  p.remote,
  COALESCE(round(p.salary_usd_annual), 0)::bigint AS salary_usd_annual,
  p.currency_inferred,
  p.posted_on
FROM outbox o
JOIN postings p ON p.id = o.posting_id
WHERE o.processed_at IS NULL AND o.failed_at IS NULL
ORDER BY o.id
LIMIT $1
FOR UPDATE OF o SKIP LOCKED;

-- name: MarkProcessed :exec
UPDATE outbox
SET processed_at = now()
WHERE id = ANY($1::bigint[]);

-- name: MarkFailed :exec
UPDATE outbox
SET
  attempts = attempts + 1,
  last_error = $2,
  failed_at = CASE WHEN attempts + 1 >= 5 THEN now() ELSE NULL END
WHERE id = $1;

-- name: PurgeProcessed :execrows
DELETE FROM outbox
WHERE processed_at IS NOT NULL
  AND processed_at < $1;
