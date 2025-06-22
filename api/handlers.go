package api

import (
	"html/template"
	"net/http"

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

	result := instrument + "\n<input type=\"text\" placeholder=\"Enter part number\" id=\"part-number\"/>"
	w.Write([]byte(result))
}

type ImageHandler struct {
	Images map[string]pkg.ImageSet
}

func NewImageHandler() *ImageHandler {
	return &ImageHandler{
		Images: make(map[string]pkg.ImageSet),
	}
}

func (ih *ImageHandler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<div>Upload handler not implemented</div>"))
}

func Setup(ih *ImageHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", RootHandler)
	mux.Handle("/css/", web.CssServer())
	mux.HandleFunc("/instruments", InstrumentSearchHandler)
	mux.HandleFunc("/choice", ChoiceHandler)
	mux.HandleFunc("/upload", ih.UploadHandler)
	return mux
}
