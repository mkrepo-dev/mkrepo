package mkrepo

import (
	"io/fs"
	"strings"
)

func PrepareGitignores(gitignoreFS fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(gitignoreFS, ".")
	if err != nil {
		return nil, err
	}
	gitignores := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		gitignores = append(gitignores, strings.TrimSuffix(entry.Name(), ".gitignore"))
	}
	return gitignores, nil
}
