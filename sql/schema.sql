CREATE TABLE oauth2_state (
  state text PRIMARY KEY,
  verifier text,
  expires_at timestamptz NOT NULL
);

CREATE TABLE account (
  id uuid PRIMARY KEY,
  provider TEXT NOT NULL,
  email text NOT NULL,
  username text NOT NULL,
  display_name text NOT NULL,
  avatar_url text NOT NULL,
  UNIQUE(provider, email)
);

CREATE TABLE session (
  id text PRIMARY KEY,
  access_token bytea NOT NULL,
  refresh_token bytea,
  access_token_expires_at timestamptz,
  expires_at timestamptz NOT NULL,
  account_id uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE
);

-- TODO: Needs revision

CREATE TABLE template (
  id uuid PRIMARY KEY,
  name text NOT NULL, -- TODO: Maybe move to template_version and let users set this in mkrepo.yaml
  full_name text NOT NULL UNIQUE,
  url text UNIQUE,
  build_in boolean NOT NULL DEFAULT false,
  used int NOT NULL DEFAULT 0 CHECK (used >= 0),
  stars int NOT NULL DEFAULT 0 CHECK (stars >= 0),
  search tsvector GENERATED ALWAYS AS (
    to_tsvector('english', coalesce(name, '') || ' ' || coalesce(full_name, ''))
  ) STORED
);

CREATE INDEX template_search_idx ON template USING GIN (search);

CREATE TABLE template_version (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  description text,
  language text,
  version text NOT NULL,
  schema jsonb,
  template_id uuid NOT NULL REFERENCES template (id) ON DELETE CASCADE,
  UNIQUE (template_id, version)
);
