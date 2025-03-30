BEGIN;

CREATE TABLE IF NOT EXISTS "oauth2_state" (
    "state" text PRIMARY KEY,
    "expires_at" timestamp NOT NULL
);

CREATE TABLE IF NOT EXISTS "account" (
	"id" bigserial PRIMARY KEY,
	"session" text NOT NULL,
	"provider" text NOT NULL,
	"access_token" text NOT NULL,
	"refresh_token" text NOT NULL DEFAULT '',
	"expires_at" timestamp NOT NULL DEFAULT '0001-01-01 00:00:00+00'::timestamp,
	"redirect_uri" text NOT NULL,
	"email" text NOT NULL,
	"username" text NOT NULL,
	"display_name" text NOT NULL,
	"avatar_url" text NOT NULL,
	UNIQUE("session", "provider", "username")
);

CREATE TABLE IF NOT EXISTS "template" (
	"id" bigserial PRIMARY KEY,
	"name" text NOT NULL,
	"url" text NOT NULL UNIQUE,
	"stars" int NOT NULL DEFAULT 0 CHECK ("stars" >= 0)
);

CREATE TABLE IF NOT EXISTS "template_version" (
	"id" bigserial PRIMARY KEY,
	"description" text NOT NULL DEFAULT '',
	"language" text NOT NULL DEFAULT '',
	"version" text NOT NULL,
	"template_id" bigint NOT NULL REFERENCES "template" ("id") ON DELETE CASCADE,
	UNIQUE ("template_id", "version")
);

COMMIT;
