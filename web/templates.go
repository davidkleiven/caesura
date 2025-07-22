package web

import (
	"bytes"
	"embed"
	"html/template"
	"io"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/utils"
)

//go:embed templates/*
var templatesFS embed.FS

type ScoreMetaData struct {
	Composer string
	Arranger string
	Title    string
}

func Index(data *ScoreMetaData) []byte {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/index.html", "templates/header.html"))
	var buf bytes.Buffer

	deps := LoadDependencies().Dependencies
	templateData := struct {
		ScoreMetaData *ScoreMetaData
		Dependencies  *Dependencies
	}{
		ScoreMetaData: data,
		Dependencies:  &deps,
	}

	pkg.PanicOnErr(tmpl.Execute(&buf, templateData))
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

func ResourceList(w io.Writer, metaData []pkg.MetaData) {
	data := ResourceListData{
		MetaData:                 metaData,
		CheckboxVisible:          true,
		PatchVisible:             true,
		RemoveFromProjectVisible: false,
	}
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

func Projects() []byte {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/projects.html", "templates/header.html"))
	var buf bytes.Buffer
	pkg.PanicOnErr(tmpl.Execute(&buf, LoadDependencies().Dependencies))
	return buf.Bytes()
}

func ProjectList(w io.Writer, projects []pkg.Project) {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/project_list.html"))

	data := make([]struct {
		Name      string
		Id        string
		CreatedAt string
		UpdatedAt string
		NumPieces int
	}, len(projects))
	for i, project := range projects {
		data[i].Name = project.Name
		data[i].Id = project.Id()
		data[i].CreatedAt = project.CreatedAt.Format(time.RFC1123)
		data[i].UpdatedAt = project.UpdatedAt.Format(time.RFC1123)
		data[i].NumPieces = len(project.ResourceIds)
	}

	pkg.PanicOnErr(tmpl.Execute(w, data))
}

func ProjectContent(w io.Writer, project *pkg.Project, resources []pkg.MetaData) {
	resourceTable := template.Must(template.ParseFS(templatesFS, "templates/project_content.html", "templates/resource_table.html"))

	var resourceTableBuffer bytes.Buffer
	pkg.PanicOnErr(resourceTable.Execute(&resourceTableBuffer, project))

	var buffer bytes.Buffer
	rows := template.Must(template.ParseFS(templatesFS, "templates/resource_list.html"))

	data := ResourceListData{
		MetaData:                 resources,
		CheckboxVisible:          false,
		PatchVisible:             false,
		RemoveFromProjectVisible: true,
	}

	pkg.PanicOnErr(rows.Execute(&buffer, data))

	buffer.Write([]byte("</tbody>"))
	w.Write(bytes.ReplaceAll(resourceTableBuffer.Bytes(), []byte("</tbody>"), buffer.Bytes()))
}

type ResourceListData struct {
	MetaData                 []pkg.MetaData
	CheckboxVisible          bool
	PatchVisible             bool
	RemoveFromProjectVisible bool
}

type ResourceContentData struct {
	ResourceId string
	Filenames  []string
}

func ResourceContent(w io.Writer, data *ResourceContentData) {
	template := template.Must(template.ParseFS(templatesFS, "templates/resource_content.html"))
	pkg.PanicOnErr(template.Execute(w, data))
}
