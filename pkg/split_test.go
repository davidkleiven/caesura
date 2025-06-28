package pkg

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func WriteToFile(filename string, r io.Reader) error {
	outFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, r); err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filename, err)
	}
	return nil
}

func TestSplitPdf(t *testing.T) {
	var buffer bytes.Buffer

	if err := CreateNPagePdf(&buffer, 10); err != nil {
		t.Error(err)
		return
	}

	if pageCout, err := api.PageCount(bytes.NewReader(buffer.Bytes()), nil); pageCout != 10 || err != nil {
		t.Errorf("Expected 10 pages, got %d with error %v", pageCout, err)
		return
	}

	assignements := []Assignment{
		{Id: "Part1", From: 1, To: 5},
		{Id: "Part2", From: 6, To: 10},
	}

	writeFile := false
	if writeFile {
		if err := WriteToFile("test_split.pdf", &buffer); err != nil {
			t.Error(err)
			return
		}
	}

	result, err := SplitPdf(bytes.NewReader(buffer.Bytes()), assignements)
	if err != nil {
		t.Error(err)
		return
	}

	reader, err := zip.NewReader(bytes.NewReader((result.Bytes())), int64(result.Len()))
	if err != nil {
		t.Error(err)
		return
	}
	if len(reader.File) != 2 {
		t.Errorf("Expected 2 files in zip, got %d", len(reader.File))
		return
	}

	expectNames := []string{"Part1.pdf", "Part2.pdf"}
	for i, file := range reader.File {
		if file.Name != expectNames[i] {
			t.Errorf("Expected file name %s, got %s", expectNames[i], file.Name)
			return
		}
		rc, err := file.Open()
		if err != nil {
			t.Error(err)
			return
		}
		defer rc.Close()

		content, err := io.ReadAll(rc)
		if err != nil {
			t.Error(err)
			return
		}

		if pageCount, err := api.PageCount(bytes.NewReader(content), nil); pageCount != 5 || err != nil {
			t.Errorf("Expected 5 pages in %s, got %d with error %v", file.Name, pageCount, err)
			return
		}
	}
}

func TestProcessingPipelineAbortOnError(t *testing.T) {
	pipeline := &PDFPipeline{
		zipWriter: zip.NewWriter(io.Discard),
		err:       errors.New("test error"),
	}

	for _, step := range []func() *PDFPipeline{
		func() *PDFPipeline { return pipeline.ExtractPages(nil, 1, 5) },
		func() *PDFPipeline { return pipeline.WriteContext() },
		func() *PDFPipeline { return pipeline.CreateZipEntry("test") },
		func() *PDFPipeline { return pipeline.CopyToZip() },
	} {
		if step().Error() == nil {
			t.Error("Expected error to propagate through the pipeline")
			return
		}
	}
	if pipeline.Error() == nil {
		t.Error("Expected pipeline to have an error after aborting on the first step")
		return
	}
}

func TestEmptyBufferReturnedOnInvalidPdf(t *testing.T) {
	invalidPDF := bytes.NewBufferString("This is not a valid PDF content")
	assignments := []Assignment{}

	result, err := SplitPdf(bytes.NewReader(invalidPDF.Bytes()), assignments)
	if err == nil {
		t.Error("Expected an error for invalid PDF content, but got none")
		return
	}

	// Ensure empty buffer is returned on error
	if result.Len() == 0 {
		t.Errorf("Expected empty buffer on error, got %d bytes", result.Len())
	}
}

func TestProcessingAbortOnError(t *testing.T) {
	var buffer bytes.Buffer

	if err := CreateNPagePdf(&buffer, 10); err != nil {
		t.Error(err)
		return
	}

	assignments := []Assignment{
		{Id: "Part1", From: 1000, To: 1500},
	}
	result, err := SplitPdf(bytes.NewReader(buffer.Bytes()), assignments)
	if err == nil {
		t.Error("Expected an error due to invalid assignment ID, but got none")
		return
	}

	if result.Len() == 0 {
		t.Errorf("Expected non-empty buffer on error, got %d bytes", result.Len())
		return
	}

	if !strings.Contains(err.Error(), "failed to process assignment") {
		t.Errorf("Expected error about invalid assignment ID, got: %v", err)
		return
	}
}
