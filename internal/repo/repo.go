package repo

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

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/provider"
	"github.com/FilipSolich/mkrepo/internal/template"
)

// Create remote repo and initialize it if needed. Returns url to the repo.
func CreateNewRepo(ctx context.Context, repo internal.Repo, provider provider.ProviderClient) (string, error) {
	username, email, err := provider.GetGitAuthor(ctx)
	if err != nil {
		return "", err
	}
	repo.AuthorEmail = email
	repo.AuthorName = username

	url, cloneUrl, err := provider.CreateRemoteRepo(ctx, repo)
	if err != nil {
		return "", err
	}

	if !repo.NeedInitialization() {
		return url, nil
	}

	// TODO: Wait with context
	err = initializeRepo(repo, cloneUrl)
	if err != nil {
		// TODO: Delete remote repo that cannot be initialized?
		return url, err
	}

	// TODO: For template repo register webhook

	return url, nil
}

func initializeRepo(repo internal.Repo, cloneUrl string) error {
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

	err = addFiles(repo, dir)
	if err != nil {
		return err
	}

	signature := &object.Signature{Name: repo.AuthorName, Email: repo.AuthorEmail, When: time.Now()}
	err = wt.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		return err
	}
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
	return r.Push(&git.PushOptions{
		FollowTags: true, // TODO: Does this do what I think it does?
		Auth:       &githttp.BasicAuth{Username: "mkrepo", Password: repo.AuthToken},
	})
}

func addFiles(repo internal.Repo, dir string) error {
	if repo.IsTemplate {
		return addTemplateFiles(repo, dir)
	}

	if repo.Readme {
		err := addReadme(repo, dir)
		if err != nil {
			return err
		}
	}

	return nil
}

func addTemplateFiles(repo internal.Repo, dir string) error {
	context := template.TemplateContext{
		Name: repo.Name,
	}
	sub, err := fs.Sub(template.RepoFS, "template")
	if err != nil {
		return err
	}
	return template.ExecuteTemplateRepo(sub, dir, context, true)
}

func addReadme(repo internal.Repo, dir string) error {
	return template.CreateFile(filepath.Join(dir, "README.md"), template.Readme, template.ReadmeContext{Name: repo.Name})
}
