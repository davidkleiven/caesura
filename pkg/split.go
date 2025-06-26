package pkg

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
)

type Assignment struct {
	Id   string `json:"id"`
	From int    `json:"from"`
	To   int    `json:"to"`
}

func SplitPdf(rs io.ReadSeeker, assignments []Assignment) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	defer zipWriter.Close()

	ctx, err := api.ReadContext(rs, nil)
	if err != nil {
		return &buf, fmt.Errorf("failed to read PDF context: %w", err)
	}

	for _, assignment := range assignments {
		var internalBuffer bytes.Buffer
		pages := make([]int, assignment.To-assignment.From+1)
		for i := range pages {
			pages[i] = assignment.From + i
		}

		extractedCtx, err := pdfcpu.ExtractPages(ctx, pages, false)
		if err != nil {
			return &buf, err
		}

		if err := api.WriteContext(extractedCtx, &internalBuffer); err != nil {
			return &buf, err
		}

		zipFile, err := zipWriter.Create(assignment.Id + ".pdf")
		if err != nil {
			return &buf, err
		}
		if _, err := io.Copy(zipFile, &internalBuffer); err != nil {
			return &buf, err
		}
	}
	return &buf, nil
}
