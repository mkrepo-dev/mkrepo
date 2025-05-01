package template

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"gopkg.in/yaml.v3"
)

//go:embed base
var RepoFS embed.FS

var Readme = template.Must(template.ParseFS(RepoFS, "base/README.md.tmpl"))

type ReadmeContext struct {
	Name string
}

type TemplateContext struct {
	FullName string
	Name     string
	Url      string
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

type MkrepoFile struct {
	Description *string        `json:"description,omitempty"`
	Lang        *string        `json:"lang,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// TODO: Take multiple filesystems so user can merge directory with buildin templates
func PrepareTemplates(db *database.DB, templatesFS fs.FS) error {
	entries, err := fs.ReadDir(templatesFS, ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		templateFS, err := fs.Sub(templatesFS, entry.Name())
		if err != nil {
			return err
		}

		err = prepareTemplate(db, templateFS, entry.Name())
		if err != nil {
			return err
		}
	}
	return nil
}

func prepareTemplate(db *database.DB, templatesFS fs.FS, name string) error {
	entries, err := fs.ReadDir(templatesFS, ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		templateVersionFS, err := fs.Sub(templatesFS, entry.Name())
		if err != nil {
			return err
		}

		err = prepareTemplateVersion(db, templateVersionFS, name, entry.Name())
		if err != nil {
			return err
		}
	}
	return nil
}

func prepareTemplateVersion(db *database.DB, templatesFS fs.FS, name string, version string) error {
	mkrepoFile, err := templatesFS.Open("mkrepo.yaml")
	if err != nil {
		return err
	}
	defer mkrepoFile.Close()

	var mkrepo MkrepoFile
	err = yaml.NewDecoder(mkrepoFile).Decode(&mkrepo)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.CreateTemplate(ctx, name, name, nil, version, mkrepo.Description, mkrepo.Lang, true)
	if err != nil && !errors.Is(err, database.ErrAlreadyExists) {
		return err
	}

	slog.Debug("Template prepared", "name", name, "version", version)
	return nil
}
