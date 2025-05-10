package mkrepo_test

import (
	"testing"

	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/template/gitignore"
	"github.com/spf13/afero"
)

func TestPrepareGitignore(t *testing.T) {
	t.Parallel()
	cmp := func(t *testing.T, want map[string]struct{}, got []string) {
		t.Helper()
		for _, g := range got {
			if _, ok := want[g]; !ok {
				t.Errorf("Unexpected gitignore: %s", g)
			}
			delete(want, g)
		}
		if len(want) > 0 {
			t.Fatalf("Missing gitignore: %v", want)
		}
	}

	t.Run("BuildIn", func(t *testing.T) {
		want := map[string]struct{}{
			"Go": {},
		}
		got, err := mkrepo.PrepareGitignores(gitignore.FS)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		cmp(t, want, got)
	})
	t.Run("IgnoreSubDirs", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		err := afero.WriteFile(fs, "test/.gitignore", []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		err = afero.WriteFile(fs, "Go.gitignore", []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		want := map[string]struct{}{"Go": {}}
		got, err := mkrepo.PrepareGitignores(afero.NewIOFS(fs))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		cmp(t, want, got)
	})
}
