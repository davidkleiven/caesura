package web

import (
	"embed"
	"net/http"
)

//go:embed js/*
var jsFS embed.FS

func JsServer() http.Handler {
	return http.FileServer(http.FS(jsFS))
}
