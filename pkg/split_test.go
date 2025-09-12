package pkg

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
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
		t.Fatalf("failed to create pdf: %s", err)

	}

	if pageCout, err := api.PageCount(bytes.NewReader(buffer.Bytes()), nil); pageCout != 10 || err != nil {
		t.Fatalf("Expected 10 pages, got %d with error %v", pageCout, err)

	}

	assignements := []Assignment{
		{Id: "Part1", From: 1, To: 5},
		{Id: "Part2", From: 6, To: 10},
	}

	writeFile := false
	if writeFile {
		if err := WriteToFile("test_split.pdf", &buffer); err != nil {
			t.Fatal(err)

		}
	}

	pdfIter := SplitPdf(bytes.NewReader(buffer.Bytes()), assignements)

	expectNames := []string{"Part1.pdf", "Part2.pdf"}
	count := 0
	for name, content := range pdfIter {
		if name != expectNames[count] {
			t.Fatalf("Expected file name %s, got %s", expectNames[count], name)

		}

		if pageCount, err := api.PageCount(bytes.NewReader(content), nil); pageCount != 5 || err != nil {
			t.Fatalf("Expected 5 pages in %s, got %d with error %v", name, pageCount, err)

		}
		count++
	}
}

func TestProcessingPipelineAbortOnError(t *testing.T) {
	pipeline := &PDFPipeline{
		err: errors.New("test error"),
	}

	for _, step := range []func() *PDFPipeline{
		func() *PDFPipeline { return pipeline.ExtractPages(nil, 1, 5) },
		func() *PDFPipeline { pipeline.WriteContext(); return pipeline },
	} {
		if step().Error() == nil {
			t.Fatal("Expected error to propagate through the pipeline")

		}
	}
	if pipeline.Error() == nil {
		t.Fatal("Expected pipeline to have an error after aborting on the first step")

	}
}

func TestEmptyBufferReturnedOnInvalidPdf(t *testing.T) {
	invalidPDF := bytes.NewBufferString("This is not a valid PDF content")
	assignments := []Assignment{}

	pdfIter := SplitPdf(bytes.NewReader(invalidPDF.Bytes()), assignments)

	num := 0
	for range pdfIter {
		num++
	}

	// Ensure empty buffer is returned on error
	if num != 0 {
		t.Fatalf("Expected empty buffer on error, got %d bytes", num)
	}
}

func TestProcessingAbortOnError(t *testing.T) {
	var buffer bytes.Buffer

	if err := CreateNPagePdf(&buffer, 10); err != nil {
		t.Fatal(err)

	}

	assignments := []Assignment{
		{Id: "Part1", From: 1000, To: 1500},
	}
	pdfIter := SplitPdf(bytes.NewReader(buffer.Bytes()), assignments)

	num := 0
	for range pdfIter {
		num++
	}

	if num != 0 {
		t.Fatalf("Expected 0 files got %d", num)

	}

}
