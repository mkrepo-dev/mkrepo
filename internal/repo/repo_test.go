package repo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/FilipSolich/mkrepo/internal"
)

func Test_addTemplateFiles(t *testing.T) {
	repo := t.TempDir()
	type args struct {
		repo internal.Repo
		dir  string
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		wantFiles map[string][]byte
	}{
		{
			name: "Test addTemplateFiles",
			args: args{
				repo: internal.Repo{
					Name: "This is template repo",
				},
				dir: repo,
			},
			wantErr: false,
			wantFiles: map[string][]byte{
				"README.md":   []byte("# This is template repo\n\nTODO\n"),
				"mkrepo.yaml": []byte("name: This is template repo\nlang: go\n"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := addTemplateFiles(tt.args.repo, tt.args.dir); (err != nil) != tt.wantErr {
				t.Errorf("addTemplateFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
			for filename, wantContent := range tt.wantFiles {
				content, err := os.ReadFile(filepath.Join(repo, filename))
				if err != nil {
					t.Errorf("error reading file %s: %v", filename, err)
				}
				if !bytes.Equal(content, wantContent) {
					t.Errorf("content of file %s is %s, want %s", filename, content, wantContent)
				}
			}
		})
	}
}
