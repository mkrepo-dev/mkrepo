-- reverse: create index "template_search_idx" to table: "template"
DROP INDEX "template_search_idx";
-- reverse: modify "template" table
ALTER TABLE "template" DROP COLUMN "search";
