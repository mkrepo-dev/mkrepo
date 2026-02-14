-- name: GetAndDeleteOAuth2State :one
DELETE FROM oauth2_state
WHERE state = $1
RETURNING state, verifier, expires_at;

-- name: CreateOAuth2State :exec
INSERT INTO oauth2_state (state, verifier, expires_at)
VALUES ($1, $2, $3);

-- name: DeleteExpiredOAuth2States :exec
DELETE FROM oauth2_state
WHERE expires_at < now();

-- name: GetAccountBySession :one
SELECT a.id, a.provider, a.email, a.username, a.display_name, a.avatar_url,
  s.id, s.access_token, s.refresh_token, s.access_token_expires_at, s.expires_at
FROM account a JOIN session s ON a.id = s.account_id
WHERE s.id = $1;

-- name: GetAccountByProviderAndEmail :one
SELECT id, provider, email, username, display_name, avatar_url
FROM account
WHERE provider = $1 AND email = $2;

-- name: CreateAccount :exec
INSERT INTO account (id, provider, email, username, display_name, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateAccount :exec
UPDATE account
SET username = $2, display_name = $3, avatar_url = $4
WHERE id = $1;

-- name: DeleteAccount :exec
DELETE FROM account
WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO session (id, access_token, refresh_token, access_token_expires_at, expires_at, account_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateSession :exec
UPDATE session
SET access_token = $2, refresh_token = $3, access_token_expires_at = $4, expires_at = $5
WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM session
WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM session
WHERE expires_at < now();

-- name: SearchTemplates :many
SELECT t.name, t.full_name, t.url, t.build_in, t.stars, tv.version, tv.description, tv.language
FROM template t
JOIN template_version tv ON t.id = tv.template_id
WHERE tv.version = (
  SELECT version FROM template_version WHERE template_id = t.id ORDER BY version DESC LIMIT 1
) AND t.search @@ to_tsquery('english', $1)
ORDER BY t.build_in DESC, t.stars DESC
LIMIT 10;

-- name: GetTemplate :one
SELECT t.name, t.full_name, t.url, t.build_in, t.stars, tv.version, tv.description, tv.language, tv.schema
FROM template t
JOIN template_version tv ON t.id = tv.template_id
WHERE t.full_name = $1
ORDER BY tv.version DESC
LIMIT 1;

-- name: InsertTemplateIfNotExists :exec
INSERT INTO template (id, name, full_name, url, build_in)
VALUES (gen_random_uuid(), $1, $2, $3, $4)
ON CONFLICT (full_name) DO NOTHING;

-- name: InsertTemplateVersion :exec
INSERT INTO template_version (description, language, version, schema, template_id)
VALUES ($1, $2, $3, $4, (SELECT id FROM template WHERE full_name = $5));

-- name: UpdateTemplateStars :exec
UPDATE template
SET stars = $2
WHERE full_name = $1;

-- name: IncreaseTemplateUses :exec
UPDATE template
SET used = used + 1
WHERE full_name = $1;

-- name: GetValidOAuth2Verifier :one
SELECT verifier
FROM oauth2_state
WHERE state = $1 AND expires_at > now();

-- name: UpsertAccount :one
INSERT INTO account (id, provider, email, username, display_name, avatar_url)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
ON CONFLICT (provider, email) DO UPDATE
SET username = EXCLUDED.username, display_name = EXCLUDED.display_name, avatar_url = EXCLUDED.avatar_url
RETURNING id;
