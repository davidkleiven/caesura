package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

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
	for i := range n {
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
