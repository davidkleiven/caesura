package web

import (
	"embed"
	"fmt"
	"github.com/davidkleiven/caesura/utils"
	"io"
	"strings"
)

//go:embed terms/*.txt
var termsFS embed.FS

func TermsAndConditionsContent(w io.Writer, lang string) {
	var filename string
	if isNorwegian(lang) {
		filename = "terms_and_conditions_no.txt"
	} else {
		filename = "terms_and_conditions_en.txt"
	}

	file := utils.Must(termsFS.Open((fmt.Sprintf("terms/%s", filename))))
	io.Copy(w, file)
}

func isNorwegian(lang string) bool {
	return strings.Contains(lang, "no") || strings.Contains(lang, "nn") || strings.Contains(lang, "nb")
}
