package pkg

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/davidkleiven/caesura/utils"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/primitives"
)

func createTextBox(txt string) *primitives.TextBox {
	return &primitives.TextBox{
		Value:    txt,
		Position: [2]float64{100, 100},
		Font: &primitives.FormFont{
			Name: "Helvetica",
			Size: 12,
		},
	}
}

func CreateNPagePdf(w io.Writer, n int) error {
	pages := make(map[string]*primitives.PDFPage, n)
	for i := 0; i < n; i++ {
		pageNumber := i + 1
		pages[strconv.Itoa(pageNumber)] = &primitives.PDFPage{
			Content: &primitives.Content{
				TextBoxes: []*primitives.TextBox{
					createTextBox(fmt.Sprintf("This is page %d", pageNumber)),
				},
			},
		}
	}

	desc := primitives.PDF{Pages: pages}

	data, err := json.Marshal(desc)
	if err != nil {
		return err
	}
	return api.Create(nil, bytes.NewBuffer(data), w, nil)
}

// MustCreateResource creates a new resource. It panics on any error and
// is intended for setting up resources for testing purposes
func MustCreateResource(numFiles int) []byte {
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	for i := range numFiles {
		pageWriter := utils.Must(zipWriter.Create(fmt.Sprintf("Part%d.pdf", i)))
		PanicOnErr(CreateNPagePdf(pageWriter, 2))

	}
	PanicOnErr(zipWriter.Close())

	return buffer.Bytes()
}
