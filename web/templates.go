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
	pkg.PanicOnErr(tmpl.Execute(&buf, LoadDependencies().Dependencies))
	return buf.Bytes()
}

func List() []byte {
	return utils.Must(templatesFS.ReadFile("templates/list.html"))
}

func Overview() []byte {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/overview.html", "templates/header.html", "templates/resource_table.html"))
	var buf bytes.Buffer
	pkg.PanicOnErr(tmpl.Execute(&buf, LoadDependencies().Dependencies))
	return buf.Bytes()
}

func ResourceList(w io.Writer, data []pkg.MetaData) {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/resource_list.html"))
	pkg.PanicOnErr(tmpl.Execute(w, data))
}

func ProjectSelectorModal() []byte {
	return utils.Must(templatesFS.ReadFile("templates/project_selection_modal.html"))
}

func ProjectQueryInput(w io.Writer, queryContent string) {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/project_query_input.html"))
	pkg.PanicOnErr(tmpl.Execute(w, queryContent))
}
