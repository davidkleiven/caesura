package api

import (
	"context"
	"encoding/json"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/web"
)

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

	err := t.Execute(w, IdentifiedList{"instruments", instruments})
	includeError(w, http.StatusInternalServerError, "Failed to render template", err)
}

func ChoiceHandler(w http.ResponseWriter, r *http.Request) {
	instrument := r.URL.Query().Get("instrument")

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

type StoreManager struct {
	Store   pkg.Storer
	Timeout time.Duration
}

func (s *StoreManager) SubmitHandler(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(r.Context(), s.Timeout)
	defer cancel()

	if err := s.Submit(ctx, &metaData, buf); err != nil {
		http.Error(w, "Failed to store file", http.StatusInternalServerError)
		slog.Error("Failed to store file", "error", err)
		return
	}
	slog.Info("File stored successfully", "filename", resourceName)
	w.Write([]byte("File uploaded successfully!"))
}

func (s *StoreManager) Submit(ctx context.Context, meta *pkg.MetaData, r io.Reader) error {
	done := make(chan error, 1)

	go func() {
		if err := s.Store.Register(meta); err != nil {
			slog.Error("Failed to register metadata", "error", err)
			done <- err
			return
		}

		name := meta.ResourceName()
		if err := s.Store.Store(name, r); err != nil {
			slog.Error("Failed to store file", "error", err)
			done <- err
			return
		}

		if err := s.Store.RegisterSuccess(meta.ResourceId()); err != nil {
			slog.Error("Failed to register success", "error", err)
			done <- err
			return
		}
		slog.Info("File submitted successfully", "filename", name, "metadata", meta)
		done <- nil
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

type FetchManager struct {
	Fetcher pkg.Fetcher
	Timeout time.Duration
}

func (fm *FetchManager) OverviewSearchHandler(w http.ResponseWriter, r *http.Request) {
	filterValue := r.URL.Query().Get("resource-filter")
	pattern := &pkg.MetaData{
		Title:    filterValue,
		Composer: filterValue,
		Arranger: filterValue,
	}

	ctx, cancel := context.WithTimeout(r.Context(), fm.Timeout)
	defer cancel()
	meta, err := fm.Meta(ctx, pattern)
	if err != nil {
		http.Error(w, "Failed to fetch metadata", http.StatusInternalServerError)
		slog.Error("Failed to fetch metadata", "error", err)
		return
	}
	web.ResourceList(w, meta)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func (fm *FetchManager) Meta(ctx context.Context, pattern *pkg.MetaData) ([]pkg.MetaData, error) {
	done := make(chan []pkg.MetaData, 1)
	errChan := make(chan error, 1)

	go func() {
		meta, err := fm.Fetcher.Meta(pattern)
		if err != nil {
			slog.Error("Failed to fetch metadata", "error", err)
			errChan <- err
			return
		}
		done <- meta
	}()

	select {
	case <-ctx.Done():
		return []pkg.MetaData{}, ctx.Err()
	case meta := <-done:
		return meta, nil
	case err := <-errChan:
		return []pkg.MetaData{}, err
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

func Setup(s *StoreManager, fm *FetchManager) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", RootHandler)
	mux.Handle("/css/", web.CssServer())
	mux.HandleFunc("/instruments", InstrumentSearchHandler)
	mux.HandleFunc("/choice", ChoiceHandler)
	mux.HandleFunc("/js/pdf-viewer.js", JsHandler)
	mux.HandleFunc("/delete-mode", DeleteMode)
	mux.HandleFunc("/submit", s.SubmitHandler)
	mux.HandleFunc("/overview", OverviewHandler)
	mux.HandleFunc("/overview/search", fm.OverviewSearchHandler)
	return mux
}
