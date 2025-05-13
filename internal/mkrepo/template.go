package mkrepo

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mkrepo-dev/mkrepo/internal/database"
)

type MkrepoFile struct {
	Description *string        `json:"description,omitempty"`
	Lang        *string        `json:"lang,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// TODO: Take multiple filesystems so user can merge directory with buildin templates
// TODO: Retun fs with templates as root dirs without template subdir and make embed fs private
func PrepareTemplates(db *database.DB, templatesFS fs.FS) error {
	entries, err := fs.ReadDir(templatesFS, ".")
	if err != nil {
		return err
	}
	count := 0
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
		count++
	}
	slog.Info("Templates prepared", slog.Int("count", count))
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

func executeTemplateDir(dstDir string, templateFS fs.FS, context repoInitContext) error {
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
