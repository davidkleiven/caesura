package pkg

import (
	"context"
	"errors"
	"io"
	"testing"
)

func populatedDownloader() *ResourceDownloader {
	store := NewDemoStore()
	ctx := context.Background()
	return NewResourceDownloader().GetMetaData(ctx, store, store.Metadata[0].ResourceId()).GetResource(ctx, store)
}

func TestZipReaderHasFiveFiles(t *testing.T) {
	downloader := populatedDownloader()
	file, err := downloader.ZipReader()

	if err != nil {
		t.Fatal(err)
	}

	if len(file.File) != 5 {
		t.Fatalf("Expected 5 files to be present")
	}

	expect := "demotitle1_composera_arrangerx.zip"
	if name := downloader.ZipFilename(); name != expect {
		t.Fatalf("Expected filename to be '%s' got '%s'", expect, name)
	}
}

func TestNonEmptyContent(t *testing.T) {
	downloader := populatedDownloader()
	content, err := downloader.Content()
	if err != nil {
		t.Fatal(err)
	}

	contentBytes, err := io.ReadAll(content)

	if err != nil {
		t.Fatal(err)
	}

	if len(contentBytes) == 0 {
		t.Fatal("Content should not be empty")
	}
}

func TestExtractSingleFile(t *testing.T) {
	downloader := populatedDownloader()

	singleFile, err := downloader.ExtractSingleFile("Part1.pdf").FileReader()
	if err != nil {
		t.Fatal(err)
	}
	defer singleFile.Close()

	content, err := io.ReadAll(singleFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(content) == 0 {
		t.Fatal("Content of single file should not be emtpy")
	}
}

func TestResourceDownloadPropagateErrors(t *testing.T) {
	initialError := errors.New("something went wrong")
	downloader := NewResourceDownloader()
	downloader.err = initialError

	ctx := context.Background()
	store := NewInMemoryStore()

	for i, f := range []func(){
		func() { downloader.GetMetaData(ctx, store, "unknownId") },
		func() { downloader.GetResource(ctx, store) },
		func() { downloader.Content() },
		func() { downloader.ExtractSingleFile("file.pdf") },
		func() { downloader.FileReader() },
		func() { downloader.ZipReader() },
	} {
		f()
		if !errors.Is(downloader.err, initialError) {
			t.Fatalf("Test #%d: changed error state to %v", i, downloader.err)
		}
	}
}
