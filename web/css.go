package web

import (
	"embed"
	"net/http"
)

//go:embed css/*
var cssFS embed.FS

func CssServer() http.Handler {
	return http.FileServer(http.FS(cssFS))
}
