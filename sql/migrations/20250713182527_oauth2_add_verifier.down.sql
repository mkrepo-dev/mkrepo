-- reverse: modify "oauth2_state" table
ALTER TABLE "oauth2_state" DROP COLUMN "verifier";
