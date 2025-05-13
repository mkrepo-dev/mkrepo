package mkrepo

import (
	"bytes"
	"io/fs"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/mkrepo-dev/mkrepo/internal/test"
	"github.com/mkrepo-dev/mkrepo/template/template"
)

func ptr[T any](v T) *T {
	return &v
}

func TestReadmeTemplate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args repoInitContext
		want []byte
	}{
		{
			name: "WithoutDescription",
			args: repoInitContext{
				Name:        "test",
				Description: nil,
			},
			want: []byte("# test\n"),
		},
		{
			name: "WithDescription",
			args: repoInitContext{
				Name:        "test",
				Description: ptr("This is a test description"),
			},
			want: []byte("# test\n\nThis is a test description\n"),
		},
		{
			name: "WithEmptyDescription",
			args: repoInitContext{
				Name:        "test",
				Description: ptr(""),
			},
			want: []byte("# test\n\n\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bytes.Buffer
			err := readme.Execute(&got, tt.args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			diff := cmp.Diff(got.Bytes(), tt.want)
			if diff != "" {
				t.Fatalf("Content is incorrect (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestExecuteTemplateDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wantFiles := map[string][]byte{
		"go.sum":  []byte(""),
		"go.mod":  []byte("module github.com/mkrepo-dev/mkrepo\n\ngo 1.24\n"),
		"main.go": []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, 世界\")\n}\n"),
	}

	templateFS, err := fs.Sub(template.FS, "go/0.1.0")
	if err != nil {
		t.Fatalf("Cannot create sub FS: %v", err)
	}
	context := repoInitContext{
		FullName: "github.com/mkrepo-dev/mkrepo",
		Values: map[string]string{
			"goVersion": "1.24",
		},
	}
	err = executeTemplateDir(dir, templateFS, context)
	if err != nil {
		t.Fatalf("Template execution: %v", err)
	}

	test.CmpDirContent(t, dir, wantFiles)
}
