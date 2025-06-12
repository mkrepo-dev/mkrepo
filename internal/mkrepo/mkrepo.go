package mkrepo

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/metrics"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/types"
)

type repoInitContext struct {
	Name        string
	Description *string
	FullName    string
	Url         string
	Values      map[string]any
}

type RepoMaker struct {
	metrics          *metrics.Metrics
	db               *database.DB
	licenses         Licenses
	gitignores       fs.FS
	dockerfiles      Dockerfiles
	dockerignores    fs.FS
	buildInTemplates fs.FS
}

func New(metrics *metrics.Metrics, db *database.DB, gitignores fs.FS, licenses Licenses, dockerfiles Dockerfiles, dockerignores fs.FS, buildInTemplates fs.FS) *RepoMaker {
	return &RepoMaker{
		metrics:          metrics,
		db:               db,
		gitignores:       gitignores,
		licenses:         licenses,
		dockerfiles:      dockerfiles,
		dockerignores:    dockerignores,
		buildInTemplates: buildInTemplates,
	}
}

// Create remote repo and initialize it if needed. Returns url to the repo.
func (rm *RepoMaker) CreateNewRepo(ctx context.Context, client provider.Client, repo *types.CreateRepo) (string, error) {
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

	if !types.CreateRepoNeedsInitialization(repo) {
		slog.Info("Repo created")
		return remoteRepo.HtmlUrl, nil
	}

	err = rm.InitializeRepo(ctx, client, repo, remoteRepo)
	if err != nil {
		return remoteRepo.HtmlUrl, err
	}
	slog.Info("Repo created and initialized")
	rm.metrics.ReposCreated.Inc()

	if types.CreateRepoIsTemplate(repo) {
		err = client.CreateWebhook(ctx, remoteRepo)
		if err != nil {
			return remoteRepo.HtmlUrl, err
		}
		slog.Info("mkrepo template created")
	}

	return remoteRepo.HtmlUrl, nil
}

func (rm *RepoMaker) InitializeRepo(ctx context.Context, client provider.Client, repo *types.CreateRepo, remoteRepo provider.RemoteRepo) error {
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

func (rm *RepoMaker) addFiles(ctx context.Context, repo *types.CreateRepo, remoteRepo provider.RemoteRepo, dir string) error {
	context := repoInitContext{
		Name:        repo.Name,
		Description: repo.Description,
		FullName:    strings.TrimPrefix(strings.TrimPrefix(remoteRepo.HtmlUrl, "https://"), "http://"),
		Url:         remoteRepo.HtmlUrl,
	}
	if repo.Initialize.Template != nil {
		context.Values = *repo.Initialize.Template.Values
	}

	if repo.Initialize.Template != nil {
		err := rm.executeTemplateRepo(ctx, dir, repo, context)
		if err != nil {
			return err
		}
		// TODO: Decide if readme and other general files should be added based on template settings
	}

	if repo.Initialize.Readme != nil && *repo.Initialize.Readme {
		err := addReadme(dir, context)
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
		err := addLicense(dir, rm.licenses, *repo.Initialize.License)
		if err != nil {
			return err
		}
	}

	if repo.Initialize.Dockerfile != nil {
		err := addDockerfile(dir, rm.dockerfiles, *repo.Initialize.Dockerfile, context, rm.dockerignores, repo.Initialize.Dockerignore)
		if err != nil {
			return err
		}
	}

	return nil
}

func (rm *RepoMaker) executeTemplateRepo(ctx context.Context, dir string, repo *types.CreateRepo, context repoInitContext) error {
	templateInfo, err := rm.db.GetTemplate(ctx, repo.Initialize.Template.FullName, repo.Initialize.Template.Version)
	if err != nil {
		return err
	}
	var templateFS fs.FS
	if !templateInfo.BuildIn {
		// TODO: Try cache
		templateDir, err := cloneRepo(ctx, *templateInfo.Url, templateInfo.Version)
		if err != nil {
			return err
		}
		templateFS = os.DirFS(templateDir)
	} else {
		templateFS, err = fs.Sub(rm.buildInTemplates, filepath.Join(templateInfo.FullName, templateInfo.Version))
		if err != nil {
			return err
		}
	}

	return executeTemplateDir(dir, templateFS, context)
}

var readme = template.Must(template.New("").Parse("# {{.Name}}{{if .Description}}\n\n{{.Description}}{{end}}\n"))

func addReadme(dir string, context repoInitContext) error {
	return createFile(filepath.Join(dir, "README.md"), readme, context)
}

func addGitignore(dir string, gitignoreFS fs.FS, gitignoreName string) error {
	dst := filepath.Join(dir, ".gitignore")
	src := gitignoreName + ".gitignore"
	return addFile(dst, gitignoreFS, src)
}

func addLicense(dir string, licenses Licenses, createLicense types.CreateRepoInitializeLicense) error {
	license, ok := licenses[createLicense.Key]
	if !ok {
		return fmt.Errorf("license %s not found", createLicense.Key)
	}
	err := createFile(filepath.Join(dir, license.Filename), license.Template, LicenseContext{
		Year:    createLicense.Year,
		Owner:   createLicense.Fullname,
		Project: createLicense.Project,
	})
	if err != nil {
		return err
	}
	for _, licenseKey := range license.With {
		createLicense.Key = licenseKey
		err := addLicense(dir, licenses, createLicense)
		if err != nil {
			return err
		}
	}
	return nil
}

func addDockerfile(dir string, dockerfiles Dockerfiles, dockerfileName string, context repoInitContext, dockerignoresFS fs.FS, dockerignore *bool) error {
	err := createFile(filepath.Join(dir, "Dockerfile"), dockerfiles[dockerfileName].Template, context)
	if err != nil {
		return err
	}
	if dockerfiles[dockerfileName].Dockerignore && dockerignore != nil && *dockerignore {
		err = addFile(filepath.Join(dir, ".dockerignore"), dockerignoresFS, dockerfileName+".dockerignore")
	}
	return err
}

func createFile(filepath string, tmpl *template.Template, context any) error {
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
	return tmpl.Execute(f, context)
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
