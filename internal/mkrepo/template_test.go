package mkrepo_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/template/template"
)

func TestExecuteTemplateDir(t *testing.T) {
	dst := t.TempDir()
	templateFS, err := fs.Sub(template.FS, "go/0.1.0")
	if err != nil {
		t.Fatalf("Cannot create sub FS: %v", err)
	}

	context := mkrepo.TemplateContext{
		FullName: "github.com/mkrepo-dev/mkrepo",
		Values: map[string]string{
			"goVersion": "1.24",
		},
	}
	err = mkrepo.ExecuteTemplateDir(dst, templateFS, context)
	if err != nil {
		t.Fatalf("Template execution: %v", err)
	}

	expectedFiles := map[string][]byte{
		"go.mod":  []byte("module github.com/mkrepo-dev/mkrepo\n\ngo 1.24\n"),
		"main.go": []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, 世界\")\n}\n"),
	}
	for filename, content := range expectedFiles {
		file, err := os.ReadFile(filepath.Join(dst, filename))
		if err != nil {
			t.Fatalf("Reading file %s: %v", filename, err)
		}
		if !bytes.Equal(file, content) {
			t.Fatalf("Content of file %s is %s, want %s", filename, file, content)
		}
	}
}
