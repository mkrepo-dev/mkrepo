-- modify "template" table
ALTER TABLE "template" ADD COLUMN "search" tsvector NULL GENERATED ALWAYS AS (to_tsvector('english'::regconfig, ((COALESCE(name, ''::text) || ' '::text) || COALESCE(full_name, ''::text)))) STORED;
-- create index "template_search_idx" to table: "template"
CREATE INDEX "template_search_idx" ON "template" USING GIN ("search");
