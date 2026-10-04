env "local" {
  src = "file://schema.sql"
  url = "postgres://mkrepo:mkrepo@localhost:5432/mkrepo?search_path=public"
  dev = "docker://postgres/18/dev?search_path=public"
  migration {
    dir = "file://migrations"
  }
}
