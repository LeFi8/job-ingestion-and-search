-- name: InsertQuarantine :execrows
INSERT INTO quarantine (
  source_id,
  element_index,
  raw,
  error
) VALUES (
  $1, $2, $3, $4
)
ON CONFLICT (source_id, raw_hash) DO NOTHING;
