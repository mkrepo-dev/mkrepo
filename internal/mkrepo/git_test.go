package mkrepo

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

func TestCloneRepo(t *testing.T) {
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/mkrepo-dev/mkrepo"},
	})
	refs, err := rem.List(&git.ListOptions{
		// Returns all references, including peeled references.
		PeelingOption: git.AppendPeeled,
	})
	if err != nil {
		t.Fatalf("Failed to list references: %v", err)
	}
	var tags []string
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tags = append(tags, ref.Name().Short())
		}
	}

	dir, err := os.MkdirTemp("", "mkrepo-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	_, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL:           "https://github.com/mkrepo-dev/mkrepo",
		SingleBranch:  true,
		ReferenceName: plumbing.NewTagReferenceName(tags[0]),
		Depth:         1,
	})
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("Failed to clone repo: %v", err)
	}

	fmt.Println(tags)
}
