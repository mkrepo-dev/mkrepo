package handler

import (
	"net/http"

	"github.com/FilipSolich/mkrepo/internal/middleware"
	"github.com/FilipSolich/mkrepo/internal/template"
)

func getBaseContext(r *http.Request) template.BaseContext {
	accounts := middleware.Accounts(r.Context())
	return template.BaseContext{Accounts: accounts}
}
