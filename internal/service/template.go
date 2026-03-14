package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"

	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/kaptinlin/jsonschema"
	"gopkg.in/yaml.v3"
)

type MkrepoFile struct {
	Description *string            `json:"description,omitempty"`
	Lang        *string            `json:"lang,omitempty"`
	Schema      *jsonschema.Schema `json:"schema,omitempty"`
}

func PrepareTemplates(repo Repository, templatesFS fs.FS) error {
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

		err = prepareTemplate(repo, templateFS, entry.Name())
		if err != nil {
			return err
		}
		count++
	}
	slog.Info("Templates prepared", slog.Int("count", count))
	return nil
}

func prepareTemplate(repo Repository, templatesFS fs.FS, name string) error {
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

		err = prepareTemplateVersion(repo, templateVersionFS, name, entry.Name())
		if err != nil {
			return err
		}
	}
	return nil
}

func prepareTemplateVersion(repo Repository, templatesFS fs.FS, name string, version string) error {
	mkrepoFile, err := templatesFS.Open("mkrepo.yaml")
	if err != nil {
		return err
	}
	defer mkrepoFile.Close() // nolint:errcheck

	var mkrepo MkrepoFile
	err = yaml.NewDecoder(mkrepoFile).Decode(&mkrepo)
	if err != nil {
		return err
	}

	schema, err := json.Marshal(mkrepo.Schema)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = repo.CreateTemplate(ctx, name, name, nil, version, mkrepo.Description, mkrepo.Lang, schema, true)
	if err != nil {
		return err
	}

	slog.Debug("Template prepared", "name", name, "version", version)
	return nil
}

// RegisterTemplate clones a remote repo at HEAD, validates it contains mkrepo.yaml,
// and stores the template metadata in the database.
func (rm *MkrepoService) RegisterTemplate(ctx context.Context, url string, fullName string) error {
	dir, err := cloneRepo(ctx, url)
	if err != nil {
		return fmt.Errorf("clone template repo: %w", err)
	}
	defer os.RemoveAll(dir) // nolint:errcheck

	mkrepoFile, err := os.Open(filepath.Join(dir, "mkrepo.yaml"))
	if err != nil {
		return fmt.Errorf("template repo must contain mkrepo.yaml at root: %w", err)
	}
	defer mkrepoFile.Close() // nolint:errcheck

	var mf MkrepoFile
	err = yaml.NewDecoder(mkrepoFile).Decode(&mf)
	if err != nil {
		return fmt.Errorf("parse mkrepo.yaml: %w", err)
	}

	schema, err := json.Marshal(mf.Schema)
	if err != nil {
		return fmt.Errorf("marshal schema: %w", err)
	}

	name := fullName
	if idx := strings.LastIndex(fullName, "/"); idx != -1 {
		name = fullName[idx+1:]
	}

	return rm.repo.CreateTemplate(ctx, name, fullName, &url, "HEAD", mf.Description, mf.Lang, schema, false)
}

func executeTemplateDir(dstDir string, templateFS fs.FS, context templateContext) error {
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
		defer f.Close() // nolint:errcheck

		return t.Execute(f, context)
	})
	return err
}
