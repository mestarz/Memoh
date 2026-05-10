-- name: GetAppSetting :one
SELECT key, value, updated_at
FROM app_settings
WHERE key = ?;

-- name: UpsertAppSetting :one
INSERT INTO app_settings (key, value, updated_at)
VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO UPDATE
SET value = excluded.value,
    updated_at = CURRENT_TIMESTAMP
RETURNING key, value, updated_at;
