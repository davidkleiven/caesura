package web

import (
	"bytes"
	"embed"
	"html/template"
	"io"

	"github.com/davidkleiven/caesura/utils"
)

//go:embed templates/*
var templatesFS embed.FS

func Index() []byte {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/index.html", "templates/header.html"))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func List() []byte {
	return utils.Must(templatesFS.ReadFile("templates/list.html"))
}
