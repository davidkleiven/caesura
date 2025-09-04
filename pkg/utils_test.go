package pkg

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"regexp"
	"slices"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
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

func TestZipAppender(t *testing.T) {
	var (
		firstZipBuffer  bytes.Buffer
		secondZipBuffer bytes.Buffer
	)

	firstWriter := zip.NewWriter(&firstZipBuffer)
	for i := range 3 {
		fw, err := firstWriter.Create(fmt.Sprintf("file%d.txt", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte("Writer1")); err != nil {
			t.Fatal(err)
		}
	}
	firstWriter.Close()

	secondWriter := zip.NewWriter(&secondZipBuffer)
	fw, err := secondWriter.Create("file1.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("Writer2")); err != nil {
		t.Fatal(err)
	}

	fw, err = secondWriter.Create("file100.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("Writer2")); err != nil {
		t.Fatal(err)
	}
	secondWriter.Close()

	result, err := NewZipAppender().
		Add(secondZipBuffer.Bytes()).
		Add(firstZipBuffer.Bytes()).
		Merge()

	if err != nil {
		t.Fatal(err)
	}

	reader, err := zip.NewReader(bytes.NewReader(result), int64(len(result)))
	if err != nil {
		t.Fatal(err)
	}

	content := map[string]string{
		"file0.txt":   "Writer1",
		"file1.txt":   "Writer2",
		"file2.txt":   "Writer1",
		"file100.txt": "Writer2",
	}

	if len(reader.File) != len(content) {
		t.Fatalf("Expected %d files got %d", len(content), len(reader.File))
	}

	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		textContent, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}

		got, want := string(textContent), content[file.Name]
		if got != want {
			t.Fatalf("Wanted %s got %s", got, want)
		}
	}
}

func TestEmptyBytesOnErrorInZipAppender(t *testing.T) {
	expectedError := errors.New("somethig went wrong")
	appender := NewZipAppender()
	appender.err = expectedError
	result, err := appender.Add([]byte{}).Merge()
	if len(result) != 0 {
		t.Fatalf("Wanted empty byte slice got %v", result)
	}
	if !errors.Is(err, expectedError) {
		t.Fatalf("Wanted error '%s' got '%s'", expectedError, err)
	}
}

func TestRandomInsecureId(t *testing.T) {
	id := RandomInsecureID()
	pattern := regexp.MustCompile("^[a-z]+-[a-z]+-[0-9]+$")
	testutils.AssertEqual(t, pattern.Match([]byte(id)), true)
}

func TestLanguageFromReqNoAcceptLang(t *testing.T) {
	r := httptest.NewRequest("GET", "/endpoint", nil)
	testutils.AssertEqual(t, LanguageFromReq(r), "en")
}

func TestLanguageFromReqNorwegian(t *testing.T) {
	r := httptest.NewRequest("GET", "/endpoint", nil)
	r.Header.Set("Accept-Language", "nb, nn;q=0.9, en;q=0.8")
	testutils.AssertEqual(t, LanguageFromReq(r), "en")
}

func TestEnglishOnInvalidHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/endpoint", nil)
	r.Header.Set("Accept-Language", "some-random-content")
	testutils.AssertEqual(t, LanguageFromReq(r), "en")
}
