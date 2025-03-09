package template

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/provider"
)

var (
	//go:embed html
	HtmlFS embed.FS

	//go:embed template lang README.md.tmpl
	RepoFS embed.FS
)

var (
	Index       = template.Must(template.ParseFS(HtmlFS, "html/base.html", "html/index.html"))
	Login       = template.Must(template.ParseFS(HtmlFS, "html/base.html", "html/login.html"))
	NewRepoForm = template.Must(template.ParseFS(HtmlFS, "html/base.html", "html/new.html"))

	Readme = template.Must(template.ParseFS(RepoFS, "README.md.tmpl"))
)

type BaseContext struct {
	Accounts []db.Account
}

type IndexContext struct {
	BaseContext
	Providers                provider.Providers
	UnauthenticatedProviders provider.Providers
}

type LoginContext struct {
	BaseContext
	Providers provider.Providers
}

type NewRepoFormContext struct {
	BaseContext
	Name             string
	Providers        provider.Providers
	SelectedProvider string
	Owners           []provider.RepoOwner
}

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

func Render(w http.ResponseWriter, t *template.Template, context any) {
	err := t.Execute(w, context)
	if err != nil {
		slog.Error("Failed to render template", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func CreateFile(filepath string, tmpl *template.Template, context any) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, context)
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
