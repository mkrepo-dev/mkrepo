package service

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
)

type licenseContext struct {
	CopyrightYear   int
	CopyrightHolder string
}

type templateContext struct {
	Name        string
	Description *string
	FullName    string
	Url         string
	License     licenseContext
	Values      map[string]any
}

type MkrepoService struct {
	repo             Repository
	licenses         Licenses
	gitignores       fs.FS
	buildInTemplates fs.FS
}

func NewService(repo Repository, gitignores fs.FS, licenses Licenses, buildInTemplates fs.FS) *MkrepoService {
	return &MkrepoService{
		repo:             repo,
		gitignores:       gitignores,
		licenses:         licenses,
		buildInTemplates: buildInTemplates,
	}
}

// Create remote repo and initialize it if needed. Returns url to the repo.
func (rm *MkrepoService) CreateNewRepo(ctx context.Context, client provider.Client, repo *CreateRepo) (string, error) {
	remoteRepo, err := client.CreateRemoteRepo(ctx, provider.CreateRepo{
		Namespace:   repo.Namespace,
		Name:        repo.Name,
		Description: repo.Description,
		Visibility:  provider.RepoVisibility(*repo.Visibility),
		Sha256:      repo.Sha256,
	})
	if err != nil {
		return "", err
	}

	if !CreateRepoNeedsInitialization(repo) {
		slog.Info("Repo created")
		return remoteRepo.HtmlUrl, nil
	}

	err = rm.InitializeRepo(ctx, client, repo, remoteRepo)
	if err != nil {
		return remoteRepo.HtmlUrl, err
	}
	slog.Info("Repo created and initialized")

	return remoteRepo.HtmlUrl, nil
}

func (rm *MkrepoService) InitializeRepo(ctx context.Context, client provider.Client, repo *CreateRepo, remoteRepo provider.RemoteRepo) error {
	dir, err := os.MkdirTemp("", "mkrepo-")
	if err != nil {
		return err
	}
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			slog.Error("Failed to remove temporary directory", "dir", dir, "error", err)
		}
	}()

	err = rm.addFiles(ctx, repo, remoteRepo, dir)
	if err != nil {
		return err
	}

	return pushRepo(ctx, repo, dir, remoteRepo.CloneUrl, client.Token().AccessToken)
}

func (rm *MkrepoService) addFiles(ctx context.Context, repo *CreateRepo, remoteRepo provider.RemoteRepo, dir string) error {
	fs := memfs.New()
	context := templateContext{
		Name:        repo.Name,
		Description: repo.Description,
		FullName:    strings.TrimPrefix(strings.TrimPrefix(remoteRepo.HtmlUrl, "https://"), "http://"),
		Url:         remoteRepo.HtmlUrl,
	}
	if repo.Initialize.Template != nil {
		context.Values = repo.Initialize.Values
	}
	if licenseVals, ok := repo.Initialize.Values["License"]; ok {
		if licenseMap, ok := licenseVals.(map[string]string); ok {
			if year, ok := licenseMap["CopyrightYear"]; ok {
				if y, err := strconv.Atoi(year); err == nil {
					context.License.CopyrightYear = y
				}
			}
			if holder, ok := licenseMap["CopyrightHolder"]; ok {
				context.License.CopyrightHolder = holder
			}
		}
	}

	if repo.Initialize.Template != nil {
		err := rm.executeTemplateRepo(ctx, dir, repo, context)
		if err != nil {
			return err
		}
	}

	if repo.Initialize.Readme != nil && *repo.Initialize.Readme {
		err := addReadme(fs, context)
		if err != nil {
			return err
		}
	}

	if repo.Initialize.Gitignore != nil {
		err := addGitignore(dir, rm.gitignores, *repo.Initialize.Gitignore)
		if err != nil {
			return err
		}
	}

	if repo.Initialize.License != nil {
		err := AddLicense(fs, *repo.Initialize.License, rm.licenses, context)
		if err != nil {
			return err
		}
	}

	return nil
}

func (rm *MkrepoService) executeTemplateRepo(ctx context.Context, dir string, repo *CreateRepo, context templateContext) error {
	templateInfo, err := rm.repo.GetTemplate(ctx, repo.Initialize.Template.FullName)
	if err != nil {
		return err
	}
	var templateFS fs.FS
	if !templateInfo.BuildIn {
		templateDir, err := cloneRepo(ctx, *templateInfo.Url)
		if err != nil {
			return err
		}
		defer os.RemoveAll(templateDir) // nolint:errcheck
		templateFS = os.DirFS(templateDir)
	} else {
		templateFS, err = fs.Sub(rm.buildInTemplates, filepath.Join(templateInfo.FullName, templateInfo.Version))
		if err != nil {
			return err
		}
	}

	return executeTemplateDir(dir, templateFS, context)
}

var readme = template.Must(template.New("").Parse("# {{.Name}}\n{{if .Description}}\n{{.Description}}\n{{end}}"))

func addReadme(fs billy.Filesystem, context templateContext) error {
	return templateFile(fs, "README.md", 0644, readme, context)
}

func addGitignore(dir string, gitignoreFS fs.FS, gitignoreName string) error {
	dst := filepath.Join(dir, ".gitignore")
	src := gitignoreName + ".gitignore"
	return addFile(dst, gitignoreFS, src)
}

func createFile(filepath string, tmpl *template.Template, data any) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			slog.Error("Failed to close file", "file", filepath, "error", err)
		}
	}()
	return tmpl.Execute(f, data)
}

func addFile(dstFile string, srcFS fs.FS, srcFile string) error {
	f, err := srcFS.Open(srcFile)
	if err != nil {
		return err
	}
	defer f.Close() // nolint:errcheck

	err = os.MkdirAll(filepath.Dir(dstFile), 0755)
	if err != nil {
		return err
	}
	dst, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer dst.Close() // nolint:errcheck

	_, err = io.Copy(dst, f)
	return err
}

func templateFile(fs billy.Filesystem, filepath string, perm os.FileMode, tmpl *template.Template, data any) error {
	f, err := fs.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close() // nolint:errcheck
	return tmpl.Execute(f, data)
}
