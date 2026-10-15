-- name: InsertQuarantine :exec
INSERT INTO quarantine (
  source_id,
  element_index,
  raw,
  error
) VALUES (
  $1, $2, $3, $4
);
