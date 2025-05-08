package mkrepo

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	fmtconfig "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/types"
	"github.com/mkrepo-dev/mkrepo/template/template"
)

type RepoMaker struct {
	db       *database.DB
	licenses Licenses
}

func New(db *database.DB, licenses Licenses) *RepoMaker {
	return &RepoMaker{db: db, licenses: licenses}
}

// Create remote repo and initialize it if needed. Returns url to the repo.
func (rm *RepoMaker) CreateNewRepo(ctx context.Context, client provider.Client, repo *types.CreateRepo) (string, error) {
	remoteRepo, err := client.CreateRemoteRepo(ctx, provider.CreateRepo{
		Namespace:   repo.Namespace,
		Name:        repo.Name,
		Description: repo.Description,
		Visibility:  provider.RepoVisibility(*repo.Visibility),
	})
	if err != nil {
		return "", err
	}

	if !types.CreateRepoNeedsInitialization(repo) {
		slog.Info("Repo created")
		return remoteRepo.HtmlUrl, nil
	}

	// TODO: Wait with context
	err = rm.InitializeRepo(ctx, client, repo, remoteRepo)
	if err != nil {
		return remoteRepo.HtmlUrl, err
	}
	slog.Info("Repo created and initialized")

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
	defer os.RemoveAll(dir)

	err = rm.addFiles(ctx, repo, remoteRepo, dir)
	if err != nil {
		return err
	}

	return gitInitAndPush(ctx, client, repo, remoteRepo.CloneUrl, dir)
}

func gitInitAndPush(ctx context.Context, client provider.Client, repo *types.CreateRepo, cloneUrl string, dir string) error {
	initOpt := &git.PlainInitOptions{
		InitOptions: git.InitOptions{
			DefaultBranch: plumbing.Main,
		},
	}
	if repo.Sha256 != nil && *repo.Sha256 {
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

	err = wt.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		return err
	}
	signature := &object.Signature{
		Name:  repo.Initialize.Author.Name,
		Email: repo.Initialize.Author.Email,
		When:  time.Now(),
	}
	commit, err := wt.Commit("Initial commit", &git.CommitOptions{Author: signature})
	if err != nil {
		return err
	}
	_, err = r.CommitObject(commit)
	if err != nil {
		return err
	}

	if repo.Initialize.Tag != nil {
		_, err = r.CreateTag(*repo.Initialize.Tag, commit, &git.CreateTagOptions{
			Message: "Release " + *repo.Initialize.Tag,
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

func (rm *RepoMaker) addFiles(ctx context.Context, repo *types.CreateRepo, remoteRepo provider.RemoteRepo, dir string) error {
	if repo.Initialize.Template != nil {
		err := rm.executeTemplateRepo(ctx, repo, remoteRepo, dir)
		if err != nil {
			return err
		}
		// TODO: Decide if readme and other general files should be added based on template settings
	}

	if repo.Initialize.Readme != nil && *repo.Initialize.Readme {
		err := addReadme(repo.Name, dir)
		if err != nil {
			return err
		}
	}

	// TODO: Init gitignore, Dockerfile and .dockerignore

	if repo.Initialize.License != nil {
		err := rm.addLicense(*repo.Initialize.License, dir)
		if err != nil {
			return err
		}
	}

	return nil
}

func (rm *RepoMaker) executeTemplateRepo(ctx context.Context, repo *types.CreateRepo, remoteRepo provider.RemoteRepo, dir string) error {
	templateInfo, err := rm.db.GetTemplate(ctx, repo.Initialize.Template.FullName, repo.Initialize.Template.Version)
	if err != nil {
		return err
	}
	templateFS := template.FS // TODO: Get from param or struct
	if !templateInfo.BuildIn {
		// TODO: try to find localy or download and cache from git
		return errors.New("external template not implemented")
	}
	sub, err := fs.Sub(templateFS, filepath.Join(templateInfo.FullName, templateInfo.Version))
	if err != nil {
		return err
	}

	context := TemplateContext{
		Name:     repo.Name,
		FullName: strings.TrimPrefix(strings.TrimPrefix(repo.Name, "https://"), "http://"),
		Url:      remoteRepo.HtmlUrl,
		Values:   repo.Initialize.Template.Values,
	}
	return ExecuteTemplateDir(dir, sub, context)
}

func addReadme(title string, dir string) error {
	return createFile(filepath.Join(dir, "README.md"), Readme, ReadmeContext{Name: title})
}

func (rm *RepoMaker) addLicense(createLicense types.CreateRepoInitializeLicense, dir string) error {
	license, ok := rm.licenses[createLicense.Key]
	if !ok {
		return fmt.Errorf("license %s not found", createLicense.Key)
	}
	err := createFile(filepath.Join(dir, license.Filename), license.Template, LicenseContext{
		Year:     createLicense.Year,
		Fullname: createLicense.Fullname,
		Project:  createLicense.Project,
	})
	if err != nil {
		return err
	}
	for _, licenseKey := range license.With {
		createLicense.Key = licenseKey
		err := rm.addLicense(createLicense, dir)
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
