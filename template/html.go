package template

import (
	"embed"
	"log/slog"
	"net/http"
	"text/template"

	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
)

//go:embed html
var HtmlFS embed.FS

var (
	Index   = template.Must(template.ParseFS(HtmlFS, "html/base.html", "html/index.html"))
	Login   = template.Must(template.ParseFS(HtmlFS, "html/base.html", "html/login.html"))
	NewRepo = template.Must(template.ParseFS(HtmlFS, "html/base.html", "html/new.html"))
)

type BaseContext struct {
	Accounts []db.Account
}

type IndexContext struct {
	BaseContext
	Providers                provider.Providers
	UnauthenticatedProviders provider.Providers // TODO: This will probaly be needed in base context for accounts in dropdown
}

type LoginContext struct {
	BaseContext
	Providers provider.Providers
}

type NewRepoFormContext struct {
	BaseContext
	Name             string
	SelectedProvider string
	Providers        provider.Providers
	Owners           []provider.RepoOwner
	Licenses         Licenses
	CurrentYear      int
}

func Render(w http.ResponseWriter, t *template.Template, context any) {
	err := t.Execute(w, context)
	if err != nil {
		slog.Error("Failed to render template", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}
