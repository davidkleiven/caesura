package pkg

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"slices"
	"testing"
)

func TestPanicOnErr(t *testing.T) {
	err := errors.New("test error")
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("PanicOnErr did not panic on error: %v", err)
		}
	}()

	PanicOnErr(err)
}

func TestRemoveDuplicates(t *testing.T) {
	for i, test := range []struct {
		input    []string
		expected []string
	}{
		{[]string{"a", "b", "a"}, []string{"a", "b"}},
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{[]string{}, []string{}},
	} {
		t.Run(fmt.Sprintf("Test #%d", i), func(t *testing.T) {
			result := RemoveDuplicates(test.input)
			if len(result) != len(test.expected) {
				t.Errorf("Expected length %d, got %d", len(test.expected), len(result))
			}
			if slices.Compare(result, test.expected) != 0 {
				t.Errorf("Expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestFileFromZipper(t *testing.T) {
	var buffer bytes.Buffer

	zipWriter := zip.NewWriter(&buffer)
	zipWriter.Create("file1.pdf")
	zipWriter.Create("file2.pdf")
	zipWriter.Create("file3.pdf")
	zipWriter.Close()

	file, err := NewFileFromZipper().ReadBytes(&buffer).AsZip().GetFile("file2.pdf")
	if err != nil {
		t.Fatalf("Failed to extract file %s", err)
	}

	if file.Name != "file2.pdf" {
		t.Fatalf("Expected file to be named 'file2.pdf' got %s", file.Name)
	}
}

func TestFileFromZipperReadFails(t *testing.T) {
	_, err := NewFileFromZipper().ReadBytes(&failingReader{}).AsZip().GetFile("file2.pdf")
	if err == nil {
		t.Fatalf("Expected error to be returned")
	}
}

func TestFileFromZipperUnknownFile(t *testing.T) {
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	zipWriter.Create("file1.pdf")
	zipWriter.Close()

	_, err := NewFileFromZipper().ReadBytes(&buffer).AsZip().GetFile("file2.pdf")
	if !errors.Is(err, ErrFileNotInZipArchive) {
		t.Errorf("Expected error to be of type 'ErrFileNotInZipArchive' got %s", err)
	}
}
