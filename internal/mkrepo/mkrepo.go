package mkrepo

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	texttemplate "text/template"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	fmtconfig "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/template"
)

type CreateRepo struct {
	// TODO: Pass account as parameter to CreateNewRepo
	Account db.Account

	// Remote repo information
	Namespace   string
	Name        string
	Description string
	Visibility  provider.RepoVisibility

	// Initialization options
	Readme         bool
	Gitignore      string
	LicenseKey     string
	LicenseContext template.LicenseContext
	Dockerfile     string
	Dockerignore   bool

	// Extra git options
	Sha256 bool
	Tag    string

	// Rest
	IsTemplate bool

	Values any
}

func (r *CreateRepo) NeedInitialization() bool {
	return r.Readme || r.Gitignore != "" || r.Dockerfile != "" || r.LicenseKey != "" || r.IsTemplate
}

type RepoMaker struct {
	db        *db.DB
	providers provider.Providers
	licenses  template.Licenses
}

func New(db *db.DB, providers provider.Providers, licenses template.Licenses) *RepoMaker {
	return &RepoMaker{db: db, providers: providers, licenses: licenses}
}

// Create remote repo and initialize it if needed. Returns url to the repo.
func (rm *RepoMaker) CreateNewRepo(ctx context.Context, client provider.Client, repo CreateRepo) (string, error) {
	// TODO: Use types.CreateRepo instead of CreateRepo
	remoteRepo, err := client.CreateRemoteRepo(ctx, provider.CreateRepo{
		Namespace:   repo.Namespace,
		Name:        repo.Name,
		Description: repo.Description,
		Visibility:  repo.Visibility,
	})
	if err != nil {
		return "", err
	}

	if !repo.NeedInitialization() {
		slog.Info("Repo created without initialization", "url", remoteRepo.HtmlUrl)
		return remoteRepo.HtmlUrl, nil
	}

	// TODO: Wait with context
	// TODO: Wait until repo is created on remote
	err = rm.initializeRepo(ctx, repo, client, remoteRepo.CloneUrl)
	if err != nil {
		// TODO: Delete remote repo that cannot be initialized?
		return remoteRepo.HtmlUrl, err
	}
	slog.Info("Repo created and initialized", "url", remoteRepo.HtmlUrl)

	// TODO: Recognize template based on buildin template name "template"
	if repo.IsTemplate {
		err = client.CreateWebhook(ctx, remoteRepo)
		if err != nil {
			return remoteRepo.HtmlUrl, err
		}
		slog.Info("Template repo created", "url", remoteRepo.HtmlUrl)
	}

	return remoteRepo.HtmlUrl, nil
}

// TODO: Create files first than init git and push
func (rm *RepoMaker) initializeRepo(ctx context.Context, repo CreateRepo, client provider.Client, cloneUrl string) error {
	dir, err := os.MkdirTemp("", "mkrepo-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	initOpt := &git.PlainInitOptions{
		InitOptions: git.InitOptions{
			DefaultBranch: plumbing.Main,
		},
	}
	if repo.Sha256 {
		initOpt.ObjectFormat = fmtconfig.SHA256
	}
	r, err := git.PlainInitWithOptions(dir, initOpt)
	if err != nil {
		return err
	}
	wt, err := r.Worktree()
	if err != nil {
		return err
	}

	err = rm.addFiles(repo, dir)
	if err != nil {
		return err
	}

	err = wt.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		return err
	}
	signature := &object.Signature{Name: repo.Account.DisplayName, Email: repo.Account.Email, When: time.Now()}
	commit, err := wt.Commit("Initial commit", &git.CommitOptions{Author: signature})
	if err != nil {
		return err
	}
	_, err = r.CommitObject(commit)
	if err != nil {
		return err
	}

	if repo.Tag != "" {
		_, err = r.CreateTag(repo.Tag, commit, &git.CreateTagOptions{
			Message: "Release " + repo.Tag,
			Tagger:  signature,
		})
		if err != nil {
			return err
		}
	}

	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{cloneUrl},
	})
	if err != nil {
		return err
	}

	return r.PushContext(ctx, &git.PushOptions{
		FollowTags: true,
		Auth:       &githttp.BasicAuth{Username: "mkrepo", Password: client.Token().AccessToken},
	})
}

func (rm *RepoMaker) addFiles(repo CreateRepo, dir string) error {
	if repo.IsTemplate {
		return addTemplateFiles(repo, dir)
	}

	if repo.Readme {
		err := addReadme(repo, dir)
		if err != nil {
			return err
		}
	}
	if repo.LicenseKey != "" {
		err := rm.addLicense(repo, dir)
		if err != nil {
			return err
		}
	}

	return nil
}

func addTemplateFiles(repo CreateRepo, dir string) error {
	context := template.TemplateContext{
		FullName: repo.Name,
		Values:   repo.Values,
	}
	sub, err := fs.Sub(template.RepoFS, filepath.Join("template", "go", "0.1.0"))
	if err != nil {
		return err
	}
	return template.ExecuteTemplateDir(dir, sub, context)
}

func addReadme(repo CreateRepo, dir string) error {
	return createFile(filepath.Join(dir, "README.md"), template.Readme, template.ReadmeContext{Name: repo.Name})
}

func (rm *RepoMaker) addLicense(repo CreateRepo, dir string) error {
	license, ok := rm.licenses[repo.LicenseKey]
	if !ok {
		return fmt.Errorf("license %s not found", repo.LicenseKey)
	}
	err := createFile(filepath.Join(dir, license.Filename), license.Template, repo.LicenseContext)
	if err != nil {
		return err
	}
	for _, licenseKey := range license.With {
		license, ok := rm.licenses[licenseKey]
		if !ok {
			return fmt.Errorf("license %s not found", licenseKey)
		}
		err := createFile(filepath.Join(dir, license.Filename), license.Template, repo.LicenseContext)
		if err != nil {
			return err
		}
	}
	return nil
}

func createFile(filepath string, tmpl *texttemplate.Template, context any) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, context)
}
