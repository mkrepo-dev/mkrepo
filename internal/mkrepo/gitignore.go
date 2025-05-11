package mkrepo

import (
	"io/fs"
	"log/slog"
	"strings"
)

func PrepareGitignores(gitignoresFS fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(gitignoresFS, ".")
	if err != nil {
		return nil, err
	}
	gitignores := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		gitignore := strings.TrimSuffix(entry.Name(), ".gitignore")
		gitignores = append(gitignores, gitignore)
		slog.Debug("Gitignore prepared", "name", gitignore)
	}

	slog.Info("Gitignores prepared", slog.Int("count", len(gitignores)))

	return gitignores, nil
}
