-- modify "account" table
ALTER TABLE "account" ALTER COLUMN "id" DROP DEFAULT, ALTER COLUMN "provider" TYPE text USING "provider"::text;
-- modify "template" table
ALTER TABLE "template" ALTER COLUMN "id" DROP DEFAULT;
-- drop enum type "provider"
DROP TYPE "provider";
