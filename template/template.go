package template

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed template base
var RepoFS embed.FS

var Readme = template.Must(template.ParseFS(RepoFS, "base/README.md.tmpl"))

type ReadmeContext struct {
	Name string
}

type TemplateContext struct {
	FullName string
	Name     string
	Values   any
}

func ExecuteTemplateDir(dstDir string, templateFS fs.FS, context TemplateContext) error {
	err := fs.WalkDir(templateFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "mkrepo.yaml" {
			return nil
		}

		t, err := template.ParseFS(templateFS, path)
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
