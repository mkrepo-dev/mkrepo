package mkrepo_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
)

func TestExecuteTemplateDir(t *testing.T) {
	dstDir := t.TempDir()
	testTemplateFS := os.DirFS("template/go/0.1.0")
	context := mkrepo.TemplateContext{
		FullName: "github.com/mkrepo-dev/mkrepo",
		Values: map[string]string{
			"goVersion": "1.24",
		},
	}

	err := mkrepo.ExecuteTemplateDir(dstDir, testTemplateFS, context)
	if err != nil {
		t.Fatalf("template execution: %v", err)
	}

	expectedFiles := map[string][]byte{
		"go.mod":  []byte("module github.com/mkrepo-dev/mkrepo\n\ngo 1.24\n"),
		"main.go": []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, 世界\")\n}\n"),
	}
	for filename, content := range expectedFiles {
		file, err := os.ReadFile(filepath.Join(dstDir, filename))
		if err != nil {
			t.Fatalf("reading file %s: %v", filename, err)
		}
		if !bytes.Equal(file, content) {
			t.Fatalf("content of file %s is %s, want %s", filename, file, content)
		}
	}
}
