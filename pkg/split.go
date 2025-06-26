package pkg

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
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
		pdfProcessor := &PDFPipeline{zipWriter: zipWriter}
		pdfProcessor.ExtractPages(ctx, assignment.From, assignment.To).
			WriteContext().
			CreateZipEntry(assignment.Id).
			CopyToZip()

		if err := pdfProcessor.Error(); err != nil {
			return &buf, fmt.Errorf("failed to process assignment %s: %w", assignment.Id, err)
		}
	}
	return &buf, nil
}

type PDFPipeline struct {
	ctx            *model.Context
	internalBuffer bytes.Buffer
	zipWriter      *zip.Writer
	zipFile        io.Writer
	err            error
}

func (p *PDFPipeline) ExtractPages(ctx *model.Context, fromPage int, toPage int) *PDFPipeline {
	if p.err != nil {
		return p
	}

	pages := make([]int, toPage-fromPage+1)
	for i := fromPage; i <= toPage; i++ {
		pages[i-fromPage] = i
	}
	p.ctx, p.err = pdfcpu.ExtractPages(ctx, pages, false)
	return p
}

func (p *PDFPipeline) WriteContext() *PDFPipeline {
	if p.err != nil {
		return p
	}
	p.err = api.WriteContext(p.ctx, &p.internalBuffer)
	return p
}

func (p *PDFPipeline) CreateZipEntry(assignmentID string) *PDFPipeline {
	if p.err != nil {
		return p
	}
	p.zipFile, p.err = p.zipWriter.Create(assignmentID + ".pdf")
	return p
}

func (p *PDFPipeline) CopyToZip() *PDFPipeline {
	if p.err != nil {
		return p
	}
	_, p.err = io.Copy(p.zipFile, &p.internalBuffer)
	return p
}

func (p *PDFPipeline) Error() error {
	return p.err
}
