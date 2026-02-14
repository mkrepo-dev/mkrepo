-- create enum type "provider"
CREATE TYPE "provider" AS ENUM ('github', 'gitlab', 'gitea');
-- create "account" table
CREATE TABLE "account" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "provider" "provider" NOT NULL,
  "email" text NOT NULL,
  "username" text NOT NULL,
  "display_name" text NOT NULL,
  "avatar_url" text NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "account_provider_email_key" UNIQUE ("provider", "email")
);
-- create "oauth2_state" table
CREATE TABLE "oauth2_state" (
  "state" text NOT NULL,
  "verifier" text NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("state")
);
-- create "session" table
CREATE TABLE "session" (
  "session" text NOT NULL,
  "access_token" bytea NOT NULL,
  "refresh_token" bytea NULL,
  "access_token_expires_at" timestamptz NULL,
  "expires_at" timestamptz NOT NULL,
  "account_id" uuid NOT NULL,
  PRIMARY KEY ("session"),
  CONSTRAINT "session_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "account" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create "template" table
CREATE TABLE "template" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "name" text NOT NULL,
  "full_name" text NOT NULL,
  "url" text NULL,
  "build_in" boolean NOT NULL DEFAULT false,
  "used" integer NOT NULL DEFAULT 0,
  "stars" integer NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "template_full_name_key" UNIQUE ("full_name"),
  CONSTRAINT "template_url_key" UNIQUE ("url"),
  CONSTRAINT "template_stars_check" CHECK (stars >= 0),
  CONSTRAINT "template_used_check" CHECK (used >= 0)
);
-- create "template_version" table
CREATE TABLE "template_version" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "description" text NULL,
  "language" text NULL,
  "version" text NOT NULL,
  "schema" jsonb NULL,
  "template_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "template_version_template_id_version_key" UNIQUE ("template_id", "version"),
  CONSTRAINT "template_version_template_id_fkey" FOREIGN KEY ("template_id") REFERENCES "template" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
