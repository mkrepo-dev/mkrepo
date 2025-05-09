-- create "account" table
CREATE TABLE "account" ("id" bigserial NOT NULL, "provider" text NOT NULL, "access_token" text NOT NULL, "refresh_token" text NULL, "expires_at" timestamp NULL, "email" text NOT NULL, "username" text NOT NULL, "display_name" text NOT NULL, "avatar_url" text NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "account_provider_username_key" UNIQUE ("provider", "username"));
-- create "oauth2_state" table
CREATE TABLE "oauth2_state" ("state" text NOT NULL, "expires_at" timestamp NOT NULL, PRIMARY KEY ("state"));
-- create "session" table
CREATE TABLE "session" ("session" text NOT NULL, "expires_at" timestamp NOT NULL, "account_id" bigint NOT NULL, PRIMARY KEY ("session"), CONSTRAINT "session_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "account" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- create "template" table
CREATE TABLE "template" ("id" bigserial NOT NULL, "name" text NOT NULL, "full_name" text NOT NULL, "url" text NULL, "build_in" boolean NOT NULL DEFAULT false, "used" integer NOT NULL DEFAULT 0, "stars" integer NOT NULL DEFAULT 0, PRIMARY KEY ("id"), CONSTRAINT "template_full_name_key" UNIQUE ("full_name"), CONSTRAINT "template_url_key" UNIQUE ("url"), CONSTRAINT "template_stars_check" CHECK (stars >= 0), CONSTRAINT "template_used_check" CHECK (used >= 0));
-- create "template_version" table
CREATE TABLE "template_version" ("id" bigserial NOT NULL, "description" text NULL, "language" text NULL, "version" text NOT NULL, "template_id" bigint NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "template_version_template_id_version_key" UNIQUE ("template_id", "version"), CONSTRAINT "template_version_template_id_fkey" FOREIGN KEY ("template_id") REFERENCES "template" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
