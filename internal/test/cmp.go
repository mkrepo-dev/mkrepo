package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func CmpDirContent(t *testing.T, dir string, wantFiles map[string][]byte) {
	t.Helper()
	for filename, content := range wantFiles {
		file, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil {
			t.Fatalf("Reading file %s: %v", filename, err)
		}
		diff := cmp.Diff(string(file), string(content))
		if diff != "" {
			t.Fatalf("Content of file %s is incorrect (-want, +got)\n%s", filename, diff)
		}
	}
}
