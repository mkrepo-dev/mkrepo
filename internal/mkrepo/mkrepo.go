package mkrepo

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	fmtconfig "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/provider"
	"github.com/FilipSolich/mkrepo/template"
)

type RepoMaker struct {
	db       *db.DB
	licenses template.Licenses
}

func New(db *db.DB, licenses template.Licenses) *RepoMaker {
	return &RepoMaker{db: db, licenses: licenses}
}

// Create remote repo and initialize it if needed. Returns url to the repo.
func (rm *RepoMaker) CreateNewRepo(ctx context.Context, repo CreateRepo, prov provider.Provider) (string, error) {
	// TODO: Put dependencies as DB in struct
	client, token := prov.NewClient(ctx, repo.Account.Token, repo.Account.RedirectUri)
	repo.Account.Token = token
	err := rm.db.UpdateAccountToken(ctx, repo.Account.Session, repo.Account.Provider, repo.Account.Username, repo.Account.Token)
	if err != nil {
		return "", err
	}
	url, cloneUrl, err := client.CreateRemoteRepo(ctx, provider.CreateRepo{
		Namespace:   repo.Namespace,
		Name:        repo.Name,
		Description: repo.Description,
		Visibility:  repo.Visibility,
	})
	if err != nil {
		return "", err
	}

	if !repo.NeedInitialization() {
		return url, nil
	}

	// TODO: Wait with context
	// TODO: Wait until repo is created on remote
	err = rm.initializeRepo(ctx, repo, prov, cloneUrl)
	if err != nil {
		// TODO: Delete remote repo that cannot be initialized?
		return url, err
	}

	// TODO: For template repo register webhook

	return url, nil
}

func (rm *RepoMaker) initializeRepo(ctx context.Context, repo CreateRepo, provider provider.Provider, cloneUrl string) error {
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
	ts := provider.OAuth2Config(repo.Account.RedirectUri).TokenSource(ctx, repo.Account.Token)
	token, err := ts.Token()
	if err != nil {
		return err
	}
	repo.Account.Token = token
	err = rm.db.UpdateAccountToken(ctx, repo.Account.Session, repo.Account.Provider, repo.Account.Username, repo.Account.Token)
	if err != nil {
		return err
	}

	return r.Push(&git.PushOptions{
		FollowTags: true,
		Auth:       &githttp.BasicAuth{Username: "mkrepo", Password: repo.Account.Token.AccessToken},
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
	if repo.LicenseKey != "none" {
		err := template.AddLicense(rm.licenses, repo.LicenseKey, repo.LicenseContext, dir)
		if err != nil {
			return err
		}
	}

	return nil
}

func addTemplateFiles(repo CreateRepo, dir string) error {
	context := template.TemplateContext{
		Name: repo.Name,
		Lang: "go", // TODO: Take from repo later
	}
	sub, err := fs.Sub(template.RepoFS, "template")
	if err != nil {
		return err
	}
	return template.ExecuteTemplateRepo(sub, dir, context, true)
}

func addReadme(repo CreateRepo, dir string) error {
	return template.CreateFile(filepath.Join(dir, "README.md"), template.Readme, template.ReadmeContext{Name: repo.Name})
}
