package mkrepo

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"

	"github.com/mkrepo-dev/mkrepo/internal/test"
)

func Test_addFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wantFiles := map[string][]byte{
		"test.txt":     []byte("test"),
		"dir/test.txt": []byte("dirtest"),
	}

	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	err = afero.WriteFile(fs, "dir/test.txt", []byte("dirtest"), 0644)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	err = addFile(filepath.Join(dir, "test.txt"), afero.NewIOFS(fs), "test.txt")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	err = addFile(filepath.Join(dir, "dir/test.txt"), afero.NewIOFS(fs), filepath.Join("dir/test.txt"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	test.CmpDirContent(t, dir, wantFiles)
}
