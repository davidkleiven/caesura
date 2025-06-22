package api

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/davidkleiven/caesura/config"
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

type ImageList struct {
	Number        int
	TailwindClass string
}

type ImageHandler struct {
	Images map[string]*pkg.ImageSet
	Config *config.Config
}

func WithConfig(config *config.Config) func(ih *ImageHandler) {
	return func(ih *ImageHandler) {
		ih.Config = config
	}
}

func NewImageHandler(opts ...func(ih *ImageHandler)) *ImageHandler {
	imgHandler := &ImageHandler{
		Images: make(map[string]*pkg.ImageSet),
		Config: config.NewDefaultConfig(),
	}

	for _, opt := range opts {
		opt(imgHandler)
	}
	return imgHandler
}

func (ih *ImageHandler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	// Protect against very large files
	if err := r.ParseMultipartForm(int64(ih.Config.MaxRequestSizeMB) << 20); err != nil {
		http.Error(w, fmt.Sprintf("The file is too large (max. file size %d MB): ", ih.Config.MaxRequestSizeMB), http.StatusBadRequest)
		slog.Error("Failed to parse multipart form", "error", err)
		return
	}

	f, header, err := r.FormFile("file-upload")
	if err != nil {
		http.Error(w, "Failed to retrieve file from form: "+err.Error(), http.StatusBadRequest)
		slog.Error("Failed to retrieve file from form", "error", err)
		return
	}

	slog.Info("Received file upload", "filename", header.Filename, "size", header.Size)
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "Failed to read file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	images, err := pkg.ProcessImages(bytes.NewReader(buf), ih.Config.AdaptiveGaussian)
	if err != nil {
		http.Error(w, "Failed to process images: "+err.Error(), http.StatusInternalServerError)
		return
	}
	ih.Images[r.RemoteAddr] = images

	html := string(web.Scans())
	t := template.Must(template.New("scans").Parse(html))
	imgList := make([]ImageList, len(images.Images))
	for i := range imgList {
		imgList[i].Number = i
		if i == 0 {
			imgList[i].TailwindClass = "block"
		} else {
			imgList[i].TailwindClass = "hidden"
		}
	}
	err = t.Execute(w, imgList)
	includeError(w, http.StatusInternalServerError, "Failed to render template", err)
}

func (ih *ImageHandler) ScansHandler(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	if page == "" {
		slog.Error("No page passed using 1")
		page = "0"
	}
	pageNum, err := strconv.Atoi(page)
	if err != nil {
		http.Error(w, "Invalid page number: "+err.Error(), http.StatusBadRequest)
		return
	}
	images, ok := ih.Images[r.RemoteAddr]
	if !ok {
		http.Error(w, "No images found for this session", http.StatusNotFound)
		return
	}
	if pageNum < 0 || pageNum >= len(images.Images) {
		http.Error(w, fmt.Sprintf("Page number out of range: %d (total pages: %d)", pageNum, len(images.Images)), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	img := images.Images[pageNum]
	if _, err := w.Write(img.Bytes()); err != nil {
		http.Error(w, "Failed to write image: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func Setup(ih *ImageHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", RootHandler)
	mux.Handle("/css/", web.CssServer())
	mux.HandleFunc("/instruments", InstrumentSearchHandler)
	mux.HandleFunc("/choice", ChoiceHandler)
	mux.HandleFunc("/upload", ih.UploadHandler)
	mux.HandleFunc("/scans", ih.ScansHandler)
	return mux
}
