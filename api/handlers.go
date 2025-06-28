package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

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
	Store pkg.Storer
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
	for _, rawAssignment := range raw {
		var assignment pkg.Assignment
		slog.Info("Processing assignment", "rawAssignment", rawAssignment)
		if err := json.Unmarshal([]byte(rawAssignment), &assignment); err != nil {
			http.Error(w, "Failed to parse assignments", http.StatusBadRequest)
			slog.Error("Failed to parse assignments", "error", err)
			return
		}
		assignments = append(assignments, assignment)
	}

	buf, err := pkg.SplitPdf(file, assignments)
	if err != nil {
		http.Error(w, "Failed to split PDF", http.StatusInternalServerError)
		slog.Error("Failed to split PDF", "error", err)
		return
	}

	filename := fmt.Sprintf("%s_%s.zip", r.FormValue("title"), r.FormValue("composer"))
	s.Store.Store(strings.ReplaceAll(filename, " ", ""), buf)

	w.Write([]byte("Submission received successfully!"))
}

func Setup(s *StoreManager) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", RootHandler)
	mux.Handle("/css/", web.CssServer())
	mux.HandleFunc("/instruments", InstrumentSearchHandler)
	mux.HandleFunc("/choice", ChoiceHandler)
	mux.Handle("/js/", web.JsServer())
	mux.HandleFunc("/delete-mode", DeleteMode)
	mux.HandleFunc("/submit", s.SubmitHandler)
	return mux
}
