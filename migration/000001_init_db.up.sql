BEGIN;

CREATE TABLE IF NOT EXISTS "oauth2_state" (
    "state" text PRIMARY KEY,
    "expires_at" timestamp NOT NULL DEFAULT 'now'::timestamp
);

CREATE TABLE IF NOT EXISTS "account" (
	"id" serial PRIMARY KEY,
	"session" text NOT NULL,
	"provider" text NOT NULL,
	"access_token" text NOT NULL,
	"refresh_token" text NOT NULL,
	"expires_at" timestamp NOT NULL DEFAULT 'epoch'::timestamp,
	"redirect_uri" text NOT NULL,
	"email" text NOT NULL,
	"username" text NOT NULL,
	"display_name" text NOT NULL,
	"avatar_url" text NOT NULL,
	UNIQUE("session", "provider", "username")
);

-- CREATE TABLE IF NOT EXISTS "template" (
-- 	"id" serial PRIMARY KEY,
-- 	"name" text NOT NULL,
-- 	"url" text NOT NULL UNIQUE,
-- 	"version" text NOT NULL DEFAULT 'v0.0.0',
-- 	"stars" int NOT NULL DEFAULT 0,
-- 	"created_at" timestamp NOT NULL DEFAULT 'now'::timestamp,
-- );

COMMIT;
