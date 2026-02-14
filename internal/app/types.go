package app

import (
	"text/template"
)

type CreateRepoVisibility string

const (
	Private  CreateRepoVisibility = "private"
	Public   CreateRepoVisibility = "public"
	Internal CreateRepoVisibility = "internal"
)

type CreateRepoInitializeAuthor struct {
	Name  string
	Email string
}

type CreateRepoFile struct {
	Template *template.Template
	Values   map[string]any
}

type CreateRepoTemplate struct {
	Name    string
	Version *string
	Values  map[string]any
}

type CreateRepoInitialize struct {
	Author   CreateRepoInitializeAuthor
	Tag      *string
	Files    []CreateRepoFile
	Template *CreateRepoTemplate
}

type CreateNewRepo struct {
	Name        string
	Namespace   string
	Description *string
	Sha256      bool
	Visibility  CreateRepoVisibility
	Initialize  *CreateRepoInitialize
}
