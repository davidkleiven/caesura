package web

import (
	"embed"
	"encoding/json"
	"io"
	"text/template"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/utils"
)

//go:embed js/*
var jsFS embed.FS

func PdfJs(w io.Writer) {
	deps := LoadDependencies().Dependencies
	content := string(utils.Must(jsFS.ReadFile("js/pdf-viewer.js")))
	template := utils.Must(template.New("js").Parse(content))
	pkg.PanicOnErr(template.Execute(w, deps))
}

type JsPackages struct {
	Dependencies `json:"dependencies"`
}

type Dependencies struct {
	HtmxVersion  string `json:"htmx.org"`
	PdfJsVersion string `json:"pdfjs-dist"`
}

func LoadDependencies() JsPackages {
	var deps JsPackages
	data := utils.Must(jsFS.ReadFile("js/package.json"))
	pkg.PanicOnErr(json.Unmarshal(data, &deps))
	return deps
}
