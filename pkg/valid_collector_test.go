package pkg

import (
	"errors"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
)

type failingDocument struct{}

func (f *failingDocument) DataTo(item any) error {
	return errors.New("something went wrong")
}

func TestValidCollectorSkipFailures(t *testing.T) {
	doc1 := LocalDocument{data: MetaData{Title: "Title 1"}}
	doc2 := LocalDocument{data: MetaData{Title: "Title 2"}}
	doc3 := failingDocument{}

	documents := []Document{&doc1, &doc3, &doc2}

	collector := NewValidCollector[MetaData]()
	for _, doc := range documents {
		collector.Push(doc)
	}

	testutils.AssertEqual(t, len(collector.Items), 2)
	if collector.Err == nil {
		t.Fatal("wanted error")
	}
	testutils.AssertContains(t, collector.Err.Error(), "went wrong")
}

func TestValidCollectorNoErrorOnSuccess(t *testing.T) {
	doc1 := LocalDocument{data: MetaData{Title: "Title 1"}}
	doc2 := LocalDocument{data: MetaData{Title: "Title 2"}}

	documents := []Document{&doc1, &doc2}

	collector := NewValidCollector[MetaData]()
	for _, doc := range documents {
		collector.Push(doc)
	}

	testutils.AssertNil(t, collector.Err)
	testutils.AssertEqual(t, len(collector.Items), 2)
}
