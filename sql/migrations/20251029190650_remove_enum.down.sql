-- reverse: drop enum type "provider"
CREATE TYPE "provider" AS ENUM ('github', 'gitlab', 'gitea');
-- reverse: modify "template" table
ALTER TABLE "template" ALTER COLUMN "id" SET DEFAULT uuidv7();
-- reverse: modify "account" table
ALTER TABLE "account" ALTER COLUMN "provider" TYPE "provider", ALTER COLUMN "id" SET DEFAULT uuidv7();
