package mkrepo

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	fmtconfig "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/mkrepo-dev/mkrepo/internal/types"
)

func cloneRepo(ctx context.Context, url string, tag string) (string, error) {
	dir, err := os.MkdirTemp("", "mkrepo-")
	if err != nil {
		return "", err
	}
	_, err = git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:           url,
		SingleBranch:  true,
		Depth:         1,
		ReferenceName: plumbing.NewTagReferenceName(tag),
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

func pushRepo(ctx context.Context, repo *types.CreateRepo, dir string, remote string, token string) error {
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
		URLs: []string{remote},
	})
	if err != nil {
		return err
	}

	return r.PushContext(ctx, &git.PushOptions{
		FollowTags: true,
		Auth: &githttp.BasicAuth{
			Username: "mkrepo",
			Password: token,
		},
	})
}
