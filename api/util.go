package api

import (
	"net/http"
	"os"
)

func Port() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return ":" + port
}

type IdentifiedList struct {
	Id    string
	Items []string
}

func includeError(w http.ResponseWriter, status int, message string, err error) {
	if err != nil {
		http.Error(w, message+": "+err.Error(), status)
	}
}
