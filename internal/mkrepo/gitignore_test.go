package mkrepo_test

import (
	"io/fs"
	"log/slog"
	"testing"
	"testing/fstest"

	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/template/gitignore"
)

func TestPrepareGitignore(t *testing.T) {
	t.Parallel()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	tests := []struct {
		name string
		args fs.FS
		want map[string]struct{}
	}{
		{
			"BuildIn",
			gitignore.FS,
			map[string]struct{}{
				"Go": {},
			},
		},
		{
			"IgnoreSubDirs",
			fstest.MapFS{
				"test/.gitignore": {Data: []byte("test")},
				"Go.gitignore":    {Data: []byte("test")},
			},
			map[string]struct{}{
				"Go": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mkrepo.PrepareGitignores(tt.args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			for _, g := range got {
				if _, ok := tt.want[g]; !ok {
					t.Errorf("Unexpected gitignore: %s", g)
				}
				delete(tt.want, g)
			}
			if len(tt.want) > 0 {
				t.Fatalf("Missing gitignore: %v", tt.want)
			}
		})
	}
}
