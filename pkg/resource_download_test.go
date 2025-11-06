package pkg

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
)

func populatedDownloader() *ResourceDownloader {
	store := NewDemoStore()
	orgId := store.FirstOrganizationId()
	ctx := context.Background()

	resourceId := store.Data[orgId].Metadata[0].ResourceId()
	return NewResourceDownloader().GetMetaData(ctx, store, orgId, resourceId).GetResource(ctx, store, orgId)
}

func TestZipReaderHasFiveFiles(t *testing.T) {
	downloader := populatedDownloader()
	var buffer bytes.Buffer

	if err := downloader.ZipResource(&buffer, IncludeAll).Error; err != nil {
		t.Fatal(err)
	}

	file, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	testutils.AssertNil(t, err)

	if len(file.File) != 5 {
		t.Fatalf("Expected 5 files to be present")
	}

	expect := "demotitle1_composera_arrangerx.zip"
	if name := downloader.ZipFilename(); name != expect {
		t.Fatalf("Expected filename to be '%s' got '%s'", expect, name)
	}
}

func TestExtractSingleFile(t *testing.T) {
	downloader := populatedDownloader()
	var buf bytes.Buffer

	downloader.ExtractSingleFile("Part1.pdf", &buf)
	if err := downloader.Error; err != nil {
		t.Fatal(err)
	}

	if buf.Len() == 0 {
		t.Fatal("Content of single file should not be emtpy")
	}
}

func TestResourceDownloadPropagateErrors(t *testing.T) {
	initialError := errors.New("something went wrong")
	downloader := NewResourceDownloader()
	downloader.Error = initialError

	ctx := context.Background()
	store := NewMultiOrgInMemoryStore()
	orgId := "some-id"

	var buf bytes.Buffer
	for i, f := range []func(){
		func() { downloader.GetMetaData(ctx, store, orgId, "unknownId") },
		func() { downloader.GetResource(ctx, store, orgId) },
		func() { downloader.ExtractSingleFile("file.pdf", &buf) },
		func() { downloader.ZipResource(&buf, IncludeAll) },
	} {
		f()
		if !errors.Is(downloader.Error, initialError) {
			t.Fatalf("Test #%d: changed error state to %v", i, downloader.Error)
		}
	}
}

type failingWriter struct{}

func (f *failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("could not write to file")
}

func TestErrorSetOnFailingWrite(t *testing.T) {
	downloader := ResourceDownloader{
		contentIter: func(yield func(n string, b []byte) bool) {
			for range 1 {
				if !yield("name", []byte("content")) {
					return
				}
			}
		},
	}

	err := downloader.ExtractSingleFile("name", &failingWriter{}).Error
	if err == nil {
		t.Fatal("Expected error to be set")
	}
	testutils.AssertContains(t, err.Error(), "write to file")
}

func TestFilenames(t *testing.T) {
	downloader := populatedDownloader()
	want := []string{"Part0.pdf", "Part1.pdf", "Part2.pdf", "Part3.pdf", "Part4.pdf"}
	got := downloader.Filenames()
	slices.Sort(got)
	if slices.Compare(got, want) != 0 {
		t.Fatalf("Wanted %v got %v\n", want, got)
	}
}

func TestResourceNotExistOnEmpty(t *testing.T) {
	downloader := NewResourceDownloader()
	var buf bytes.Buffer
	downloader.ZipResource(&buf, IncludeAll)
	if !errors.Is(downloader.Error, ErrResourceNotFound) {
		t.Fatalf("Wanted error to be %s got %s", ErrResourceNotFound, downloader.Error)
	}
}

type failingZipWriter struct {
	errCreate error
	w         io.Writer
}

func (f *failingZipWriter) Create(name string) (io.Writer, error) {
	return f.w, f.errCreate
}

func (f *failingZipWriter) Close() error {
	return nil
}

func TestErrorSetOnFailingCreate(t *testing.T) {
	downloader := populatedDownloader()
	downloader.zwFactory = func(w io.Writer) ZipWriter {
		return &failingZipWriter{errCreate: errors.New("could not created")}
	}
	var buf bytes.Buffer
	downloader.ZipResource(&buf, IncludeAll)

	if downloader.Error == nil {
		t.Fatal("Expected error to be set")
	}

	testutils.AssertContains(t, downloader.Error.Error(), "could not")
}

func TestErrorSetOnFailingSubwriter(t *testing.T) {
	downloader := populatedDownloader()
	downloader.zwFactory = func(w io.Writer) ZipWriter {
		return &failingZipWriter{w: &failingWriter{}}
	}
	var buf bytes.Buffer
	downloader.ZipResource(&buf, IncludeAll)
	if downloader.Error == nil {
		t.Fatal("Expected error to be set")
	}

	testutils.AssertContains(t, downloader.Error.Error(), "could not write to file")
}
