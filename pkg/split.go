package pkg

import (
	"bytes"
	"io"
	"iter"
	"log/slog"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type Assignment struct {
	Id   string `json:"id"`
	From int    `json:"from"`
	To   int    `json:"to"`
}

func SplitPdf(rs io.ReadSeeker, assignments []Assignment) iter.Seq2[string, []byte] {
	return func(yield func(string, []byte) bool) {
		ctx, err := api.ReadValidateAndOptimize(rs, model.NewDefaultConfiguration())
		if err != nil {
			slog.Error("failed to read and validate PDF context", "error", err)
			return
		}

		for _, assignment := range assignments {
			pdfProcessor := &PDFPipeline{}
			buf, err := pdfProcessor.ExtractPages(ctx, assignment.From, assignment.To).WriteContext()

			if err != nil {
				slog.Error("failed to process assignment", "id", assignment.Id, "error", err)
				return
			}
			if !yield(assignment.Id+".pdf", buf.Bytes()) {
				return
			}
		}
	}
}

type PDFPipeline struct {
	ctx *model.Context
	err error
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

func (p *PDFPipeline) WriteContext() (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if p.err != nil {
		return &buf, p.err
	}

	p.err = api.WriteContext(p.ctx, &buf)
	return &buf, p.err
}

func (p *PDFPipeline) Error() error {
	return p.err
}
