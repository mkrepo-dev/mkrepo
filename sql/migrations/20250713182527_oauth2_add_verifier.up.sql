-- modify "oauth2_state" table
ALTER TABLE "oauth2_state" ADD COLUMN "verifier" text NULL;
