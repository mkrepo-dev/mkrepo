CREATE TABLE IF NOT EXISTS "oauth2_state" (
    "state" text PRIMARY KEY,
    "expires_at" timestamp NOT NULL
);

CREATE TABLE IF NOT EXISTS "account" (
	"id" bigserial PRIMARY KEY,
	"provider" text NOT NULL,
	"access_token" text NOT NULL,
	"refresh_token" text,
	"expires_at" timestamp,
	"email" text NOT NULL,
	"username" text NOT NULL,
	"display_name" text NOT NULL,
	"avatar_url" text NOT NULL,
	UNIQUE("provider", "username")
);

CREATE TABLE IF NOT EXISTS "session" (
	"session" text PRIMARY KEY,
	"expires_at" timestamp NOT NULL,
	"account_id" bigint NOT NULL REFERENCES "account" ("id") ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS "template" (
	"id" bigserial PRIMARY KEY,
	"name" text NOT NULL, -- TODO: Maybe move to tempalte_version and let users set this in mkrepo.yaml
	"full_name" text NOT NULL UNIQUE,
	"url" text UNIQUE,
	"build_in" boolean NOT NULL DEFAULT false,
	"used" int NOT NULL DEFAULT 0 CHECK ("used" >= 0),
	"stars" int NOT NULL DEFAULT 0 CHECK ("stars" >= 0)
);

CREATE TABLE IF NOT EXISTS "template_version" (
	"id" bigserial PRIMARY KEY,
	"description" text,
	"language" text,
	"version" text NOT NULL,
	"template_id" bigint NOT NULL REFERENCES "template" ("id") ON DELETE CASCADE,
	UNIQUE ("template_id", "version")
);
