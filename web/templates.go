package web

import (
	"bytes"
	"embed"
	"html/template"
	"io"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/utils"
)

//go:embed templates/*
var templatesFS embed.FS

func Index() []byte {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/index.html", "templates/header.html"))
	var buf bytes.Buffer
	pkg.PanicOnErr(tmpl.Execute(&buf, nil))
	return buf.Bytes()
}

func List() []byte {
	return utils.Must(templatesFS.ReadFile("templates/list.html"))
}
