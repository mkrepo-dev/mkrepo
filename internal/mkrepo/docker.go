package mkrepo

import (
	"io/fs"
	"log/slog"
	"strings"
	"text/template"
)

type Dockerfile struct {
	Dockerignore *string
	Template     *template.Template
}

type Dockerfiles map[string]Dockerfile

func PrepareDockerfiles(dockerfilesFS fs.FS) (Dockerfiles, error) {
	dockerfiles := make(Dockerfiles)
	entries, err := fs.ReadDir(dockerfilesFS, ".")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		key := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".Dockerfile.tmpl"), ".dockerignore")
		if strings.HasSuffix(entry.Name(), ".dockerignore") {
			filename := entry.Name()
			dockerfile := dockerfiles[key]
			dockerfile.Dockerignore = &filename
			dockerfiles[key] = dockerfile
			slog.Debug("Dockerignore prepared", "name", key)
			continue
		}

		dockerfile := dockerfiles[key]
		dockerfile.Template, err = template.ParseFS(dockerfilesFS, entry.Name())
		if err != nil {
			return nil, err
		}
		dockerfiles[key] = dockerfile
		slog.Debug("Dockerfile prepared", "name", key)
	}

	for key, dockerfile := range dockerfiles {
		if dockerfile.Template == nil {
			slog.Warn("Dockerfile template not found", "key", key)
			delete(dockerfiles, key)
		}
	}

	slog.Info("Dockerfiles prepared", slog.Int("count", len(dockerfiles)))

	return dockerfiles, nil
}
