package web

import (
	"bytes"
	"regexp"
	"testing"
)

func TestPdfJs(t *testing.T) {
	var buffer bytes.Buffer
	re := regexp.MustCompile("https://unpkg.com/pdfjs-dist@[0-9]+.[0-9]+.[0-9]+/build/pdf.min.mjs")
	PdfJs(&buffer)
	if !re.MatchString(buffer.String()) {
		t.Fatal("Expected response to have a version put into the URL for pdfjs-dist")
	}
}
