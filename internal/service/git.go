package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	gitconfig "github.com/go-git/go-git/v6/plumbing/format/config"
	"github.com/go-git/go-git/v6/plumbing/object"
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"
)

func cloneRepo(ctx context.Context, url string) (string, error) {
	dir, err := os.MkdirTemp("", "mkrepo-")
	if err != nil {
		return "", err
	}
	_, err = git.PlainCloneContext(ctx, dir, &git.CloneOptions{
		URL:          url,
		SingleBranch: true,
		Depth:        1,
	})
	if err != nil {
		err2 := os.RemoveAll(dir)
		if err2 != nil {
			return "", fmt.Errorf("%w: %w", err, err2)
		}
		return "", err
	}
	return dir, nil
}

func pushRepo(ctx context.Context, repo *CreateRepo, dir string, remote string, token string) error {
	opts := []git.InitOption{
		git.WithDefaultBranch(plumbing.Main),
	}
	if repo.Sha256 != nil && *repo.Sha256 {
		opts = append(opts, git.WithObjectFormat(gitconfig.SHA256))
	}
	r, err := git.PlainInit(dir, false, opts...)
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
		URLs: []string{remote},
	})
	if err != nil {
		return err
	}

	return r.PushContext(ctx, &git.PushOptions{
		FollowTags: true,
		ClientOptions: []client.Option{
			client.WithHTTPAuth(&githttp.TokenAuth{Token: token}),
		},
	})
}
