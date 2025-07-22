package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
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
	w.Write(web.Index(&web.ScoreMetaData{}))
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
	const maxSize = 1 << 12 // 4 kB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	err := r.ParseForm()
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		msg := "File is larger than max allowed size (~4 kB)."
		http.Error(w, msg, http.StatusRequestEntityTooLarge)
		return
	} else if err != nil {
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

func SubmitHandler(submitter pkg.Submitter, timeout time.Duration, maxSize int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		maxUploadSize := int64(maxSize) << 20

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		err := r.ParseMultipartForm(maxUploadSize)

		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			msg := fmt.Sprintf("File is larger than max allowed size (~%d MB).", maxSize)
			http.Error(w, msg, http.StatusRequestEntityTooLarge)
			return
		} else if err != nil {
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

func RemoveFromProject(remover pkg.ProjectResourceRemover, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectId := r.PathValue("projectId")
		resourceId := r.PathValue("resourceId")

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := remover.RemoveResource(ctx, projectId, resourceId); err != nil {
			http.Error(w, "failed to remove resource", http.StatusInternalServerError)
			slog.Error("Failed to remove resource", "project-id", projectId, "resource-id", resourceId, "host", r.Host)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		msg := fmt.Sprintf("Successfully deleted item %s from project %s", resourceId, projectId)
		w.Write([]byte(msg))
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
		projectId := r.PathValue("id")
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

func ResourceContentByIdHandler(s pkg.ResourceGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		id := r.PathValue("id")
		downloader := pkg.NewResourceDownloader().GetMetaData(ctx, s, id).GetResource(ctx, s)

		resource, err := downloader.ZipReader()
		if err != nil {
			http.Error(w, "could not fetch resource", http.StatusInternalServerError)
			slog.Error("Failed to fetch resource", "error", err)
		}

		content := web.ResourceContentData{
			ResourceId: id,
			Filenames:  make([]string, len(resource.File)),
		}
		for i, file := range resource.File {
			content.Filenames[i] = file.Name
		}

		web.ResourceContent(w, &content)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func ResourceDownload(s pkg.ResourceGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		resourceId := r.PathValue("id")
		filename := r.URL.Query().Get("file")
		downloader := pkg.NewResourceDownloader().GetMetaData(ctx, s, resourceId).GetResource(ctx, s)

		var (
			reader             io.Reader
			contentReader      io.ReadCloser
			err                error
			statusCode         int
			contentDisposition string
			contentType        string
		)
		if filename == "" {
			zipFilename := downloader.ZipFilename()
			contentDisposition = "attachment; filename=\"" + zipFilename + "\""
			contentType = "application/zip"
			reader, err = downloader.Content()
			contentReader = io.NopCloser(reader)
		} else {
			contentDisposition = "attachment; filename=\"" + filename + "\""
			contentType = "application/pdf"
			contentReader, err = downloader.ExtractSingleFile(filename).FileReader()
		}

		switch {
		case errors.Is(err, pkg.ErrFileNotInZipArchive),
			errors.Is(err, pkg.ErrFileNotFound),
			errors.Is(err, pkg.ErrResourceMetadataNotFound):
			statusCode = http.StatusNotFound
		case err != nil:
			statusCode = http.StatusInternalServerError
		default:
			statusCode = http.StatusOK
		}

		if err != nil {
			http.Error(w, err.Error(), statusCode)
			slog.Error("Error during download resource", "error", err, "id", resourceId, "file", filename)
			return
		}
		defer contentReader.Close()

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", contentDisposition)
		io.Copy(w, contentReader)
		slog.Info("Resource downloaded")
	}
}

func AddToResourceHanlder(metaGetter pkg.MetaByIdGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		meta, err := metaGetter.MetaById(ctx, id)
		if err != nil {
			http.Error(w, "Error when fetching metadata", http.StatusInternalServerError)
			slog.Error("Error when fetching metadata", "error", err, "id", id, "url", r.URL.Path)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(web.Index(&web.ScoreMetaData{Composer: meta.Composer, Arranger: meta.Arranger, Title: meta.Title}))
	}

}

func Setup(store pkg.BlobStore, config *pkg.Config) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", RootHandler)
	mux.Handle("/css/", web.CssServer())
	mux.HandleFunc("/instruments", InstrumentSearchHandler)
	mux.HandleFunc("/choice", ChoiceHandler)
	mux.HandleFunc("/js/pdf-viewer.js", JsHandler)
	mux.HandleFunc("/delete-mode", DeleteMode)

	mux.HandleFunc("/overview", OverviewHandler)
	mux.HandleFunc("/overview/search", OverviewSearchHandler(store, config.Timeout))
	mux.HandleFunc("/overview/project-selector", ProjectSelectorModalHandler)

	mux.HandleFunc("/project-query-input", ProjectQueryInputHandler)
	mux.Handle("/js/", web.JsServer())

	mux.HandleFunc("GET /projects", ProjectHandler)
	mux.HandleFunc("GET /projects/names", SearchProjectHandler(store, config.Timeout))
	mux.HandleFunc("GET /projects/info", SearchProjectListHandler(store, config.Timeout))
	mux.HandleFunc("GET /projects/{id}", ProjectByIdHandler(store, config.Timeout))
	mux.HandleFunc("POST /projects", ProjectSubmitHandler(store, config.Timeout))
	mux.HandleFunc("DELETE /projects/{projectId}/{resourceId}", RemoveFromProject(store, config.Timeout))

	mux.HandleFunc("GET /resources/{id}", ResourceDownload(store, config.Timeout))
	mux.HandleFunc("GET /resources/{id}/content", ResourceContentByIdHandler(store, config.Timeout))
	mux.HandleFunc("GET /resources/{id}/submit-form", AddToResourceHanlder(store, config.Timeout))
	mux.HandleFunc("POST /resources", SubmitHandler(store, config.Timeout, int(config.MaxRequestSizeMb)))
	return mux
}
