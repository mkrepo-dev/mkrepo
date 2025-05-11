package mkrepo

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/mkrepo-dev/mkrepo/internal/test"
)

func Test_addFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wantFiles := map[string][]byte{
		"test.txt":     []byte("test"),
		"dir/test.txt": []byte("dirtest"),
	}
	fs := fstest.MapFS{
		"test.txt":     {Data: []byte("test")},
		"dir/test.txt": {Data: []byte("dirtest")},
	}

	err := addFile(filepath.Join(dir, "test.txt"), fs, "test.txt")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	err = addFile(filepath.Join(dir, "dir/test.txt"), fs, filepath.Join("dir/test.txt"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	test.CmpDirContent(t, dir, wantFiles)
}
