package template

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed template lang README.md.tmpl
var RepoFS embed.FS

var (
	Readme = template.Must(template.ParseFS(RepoFS, "README.md.tmpl"))
)

type ReadmeContext struct {
	Name string
}

type TemplateContext struct {
	Name string
	Lang string
}

type GoContext struct {
	Module    string
	GoVersion string
}

func ExecuteTemplateRepo(srcFS fs.FS, dstDir string, context any, trimSuffix bool) error {
	err := fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "mkrepo.yaml" {
			return nil
		}

		t, err := template.ParseFS(srcFS, path)
		if err != nil {
			return err
		}
		f, err := os.Create(filepath.Join(dstDir, strings.TrimSuffix(path, ".tmpl")))
		if err != nil {
			return err
		}
		defer f.Close()

		return t.Execute(f, context)
	})
	return err
}
