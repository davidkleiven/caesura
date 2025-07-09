package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/web"
)

type HandlerFunc func(http.ResponseWriter, *http.Request)

func RootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(web.Index())
}

func InstrumentSearchHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	instruments := pkg.FilterList(allInstruments(), token)

	html := string(web.List())
	t := template.Must(template.New("list").Parse(html))

	err := t.Execute(w, IdentifiedList{Id: "instruments", Items: instruments, HxGet: "/choice", HxTarget: "#chosen-instrument"})
	includeError(w, http.StatusInternalServerError, "Failed to render template", err)
}

func ChoiceHandler(w http.ResponseWriter, r *http.Request) {
	instrument := r.URL.Query().Get("item")

	result := instrument + "<input type=\"text\" placeholder=\"Enter part number\" id=\"part-number\"/>"
	w.Write([]byte(result))
}

func DeleteMode(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	checkBoxValue := r.FormValue("delete-mode")
	slog.Info("Received value", "delete-mode", checkBoxValue)

	if checkBoxValue == "1" {
		w.Write([]byte("(Click to remove)"))
	} else {
		w.Write([]byte("(Click to jump)"))
	}
}

func SubmitHandler(submitter pkg.Submitter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(100 << 20); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			slog.Error("Failed to parse form", "error", err)
			return
		}

		file, _, err := r.FormFile("document")
		if err != nil {
			http.Error(w, "Failed to retrieve file from form", http.StatusBadRequest)
			slog.Error("Failed to retrieve file from form", "error", err)
			return
		}
		defer file.Close()

		var assignments []pkg.Assignment
		raw := r.MultipartForm.Value["assignments"]
		if len(raw) == 0 {
			http.Error(w, "No assignments provided", http.StatusBadRequest)
			slog.Error("No assignments provided")
			return
		}
		if err := json.Unmarshal([]byte(raw[0]), &assignments); err != nil {
			http.Error(w, "Failed to parse assignments", http.StatusBadRequest)
			slog.Error("Failed to parse assignments", "error", err)
			return
		}

		var metaData pkg.MetaData
		rawMeta := r.MultipartForm.Value["metadata"]

		if len(rawMeta) == 0 {
			http.Error(w, "No metadata provided", http.StatusBadRequest)
			slog.Error("No metadata provided")
			return
		}

		if err := json.Unmarshal([]byte(rawMeta[0]), &metaData); err != nil {
			http.Error(w, "Failed to parse metadata", http.StatusBadRequest)
			slog.Error("Failed to parse metadata", "error", err)
			return
		}

		resourceName := metaData.ResourceName()
		if resourceName == ".zip" {
			http.Error(w, "Filename is empty. Note that only alphanumeric characters are allowed", http.StatusBadRequest)
			slog.Error("Filename cannot be empty.", "title", metaData.Title, "composer", metaData.Composer, "arranger", metaData.Arranger)
			return
		}

		buf, err := pkg.SplitPdf(file, assignments)
		if err != nil {
			http.Error(w, "Failed to split PDF", http.StatusInternalServerError)
			slog.Error("Failed to split PDF", "error", err)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := submitter.Submit(ctx, &metaData, buf); err != nil {
			http.Error(w, "Failed to store file", http.StatusInternalServerError)
			slog.Error("Failed to store file", "error", err)
			return
		}
		slog.Info("File stored successfully", "filename", resourceName)
		w.Write([]byte("File uploaded successfully!"))
	}
}

func OverviewSearchHandler(fetcher pkg.MetaByPatternFetcher, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filterValue := r.URL.Query().Get("resource-filter")
		pattern := &pkg.MetaData{
			Title:    filterValue,
			Composer: filterValue,
			Arranger: filterValue,
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		meta, err := fetcher.MetaByPattern(ctx, pattern)
		if err != nil {
			http.Error(w, "Failed to fetch metadata", http.StatusInternalServerError)
			slog.Error("Failed to fetch metadata", "error", err)
			return
		}
		web.ResourceList(w, meta)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func OverviewHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(web.Overview())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func JsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	web.PdfJs(w)
}

func SearchProjectHandler(store pkg.ProjectByNameGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectName := r.URL.Query().Get("projectQuery")
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		project, err := store.ProjectsByName(ctx, projectName)
		slog.Info("Searching for projects", "project_name", projectName, "num_results", len(project))
		if err != nil {
			http.Error(w, "Failed to fetch project", http.StatusInternalServerError)
			slog.Error("Failed to fetch project", "error", err)
			return
		}

		html := string(web.List())
		t := template.Must(template.New("list").Parse(html))

		project_names := make([]string, len(project))
		for i, p := range project {
			project_names[i] = p.Name
		}
		pkg.PanicOnErr(t.Execute(w, IdentifiedList{Id: "projects", Items: project_names, HxGet: "/project-query-input", HxTarget: "#project-query-input"}))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func ProjectSelectorModalHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(web.ProjectSelectorModal())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func ProjectQueryInputHandler(w http.ResponseWriter, r *http.Request) {
	value := r.URL.Query().Get("item")
	web.ProjectQueryInput(w, value)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func ProjectSubmitHandler(submitter pkg.ProjectSubmitter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			slog.Error("Failed to parse form", "error", err)
			return
		}

		projectName := r.FormValue("projectQuery")
		if projectName == "" {
			http.Error(w, "Project name cannot be empty", http.StatusBadRequest)
			slog.Error("Project name cannot be empty")
			return
		}

		resourceIds := r.Form["pieceIds"]
		project := &pkg.Project{
			Name:        projectName,
			ResourceIds: resourceIds,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := submitter.SubmitProject(ctx, project); err != nil {
			http.Error(w, "Failed to submit project", http.StatusInternalServerError)
			slog.Error("Failed to submit project", "error", err)
			return
		}
		slog.Info("Project submitted successfully", "project_name", projectName, "num_resources", len(resourceIds))
		w.Write(fmt.Appendf(nil, "Added %d piece(s) to '%s'", len(resourceIds), projectName))
	}
}

func ProjectHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(web.Projects())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func SearchProjectListHandler(store pkg.ProjectByNameGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectName := r.URL.Query().Get("projectQuery")
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		projects, err := store.ProjectsByName(ctx, projectName)
		if err != nil {
			http.Error(w, "Failed to fetch projects", http.StatusInternalServerError)
			slog.Error("Failed to fetch projects", "error", err)
			return
		}

		web.ProjectList(w, projects)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func ProjectByIdHandler(store pkg.ProjectMetaByIdGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		interpretedPath, err := pkg.ParseUrl(r.URL.Path)
		if err != nil {
			http.Error(w, "Project ID is required", http.StatusBadRequest)
			slog.Error("Project ID is required")
			return
		}
		projectId := interpretedPath.PathParameter
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		project, err := store.ProjectById(ctx, projectId)
		if err != nil {
			http.Error(w, "Failed to fetch project", http.StatusInternalServerError)
			slog.Error("Failed to fetch project", "error", err)
			return
		}

		metaData := make([]pkg.MetaData, 0, len(project.ResourceIds))
		for _, id := range project.ResourceIds {
			meta, err := store.MetaById(ctx, id)
			if err != nil {
				slog.Error("Failed to fetch metadata for project", "error", err)
			} else {
				metaData = append(metaData, *meta)
			}
		}

		web.ProjectContent(w, project, metaData)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func ResourceContentByIdHandler(s pkg.ResourceByIder, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		interpretedPath, err := pkg.ParseUrl(r.URL.Path)
		if err != nil {
			http.Error(w, "ResourceID is required", http.StatusBadRequest)
			slog.Error("Resource ID is required", "error", err)
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		resource, err := s.ResourceById(ctx, interpretedPath.PathParameter)
		if err != nil {
			http.Error(w, "could not fetch resource", http.StatusInternalServerError)
			slog.Error("Failed to fetch resource", "error", err)
		}

		filenames := make([]string, len(resource.File))
		for i, file := range resource.File {
			filenames[i] = file.Name
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func Setup(store pkg.BlobStore, timeout time.Duration) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", RootHandler)
	mux.Handle("/css/", web.CssServer())
	mux.HandleFunc("/instruments", InstrumentSearchHandler)
	mux.HandleFunc("/choice", ChoiceHandler)
	mux.HandleFunc("/js/pdf-viewer.js", JsHandler)
	mux.HandleFunc("/delete-mode", DeleteMode)
	mux.HandleFunc("/submit", SubmitHandler(store, timeout))
	mux.HandleFunc("/overview", OverviewHandler)
	mux.HandleFunc("/overview/search", OverviewSearchHandler(store, timeout))
	mux.HandleFunc("/overview/project-selector", ProjectSelectorModalHandler)
	mux.HandleFunc("/search-projects", SearchProjectHandler(store, timeout))
	mux.HandleFunc("/add-to-project", ProjectSubmitHandler(store, timeout))
	mux.HandleFunc("/project-query-input", ProjectQueryInputHandler)
	mux.HandleFunc("/projects", ProjectHandler)
	mux.HandleFunc("/filter/project-list", SearchProjectListHandler(store, timeout))
	mux.HandleFunc("/projects/", ProjectByIdHandler(store, timeout))
	return mux
}
