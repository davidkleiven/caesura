package web

import (
	"embed"
	"encoding/json"
	"net/http"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/utils"
)

//go:embed js/*
var jsFS embed.FS

func JsServer() http.Handler {
	return http.FileServer(http.FS(jsFS))
}

type JsPackages struct {
	Dependencies `json:"dependencies"`
}

type Dependencies struct {
	HtmxVersion string `json:"htmx.org"`
}

func LoadDependencies() JsPackages {
	var deps JsPackages
	data := utils.Must(jsFS.ReadFile("js/package.json"))
	pkg.PanicOnErr(json.Unmarshal(data, &deps))
	return deps
}
