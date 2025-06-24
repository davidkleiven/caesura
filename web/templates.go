package web

import (
	"embed"

	"github.com/davidkleiven/caesura/utils"
)

//go:embed templates/*
var templatesFS embed.FS

func Index() []byte {
	return utils.Must(templatesFS.ReadFile("templates/index.html"))
}

func List() []byte {
	return utils.Must(templatesFS.ReadFile("templates/list.html"))
}

func Assignments() []byte {
	return utils.Must(templatesFS.ReadFile("templates/assignments.html"))
}
