package pkg

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func preppedInMemporyFetcher() *InMemoryStore {
	return &InMemoryStore{
		Metadata: []MetaData{
			{Title: "Test Title", Composer: "Test Composer", Arranger: "Test Arranger"},
			{Title: "Another Title", Composer: "Another Composer", Arranger: "Another Arranger"},
		},
		Data: map[string][]byte{
			"test_resource":    []byte("This is a test resource content."),
			"another_resource": []byte("This is another resource content."),
		},
	}
}

func TestFetchMeta(t *testing.T) {
	inMemFetcher := preppedInMemporyFetcher()
	for _, test := range []struct {
		name     string
		fetcher  BlobStore
		pattern  *MetaData
		expected int
	}{
		{"Empty Pattern", inMemFetcher, &MetaData{}, 2},
		{"Title Match", inMemFetcher, &MetaData{Title: "Test"}, 1},
		{"Composer Match", inMemFetcher, &MetaData{Composer: "Another Composer"}, 1},
		{"Arranger Match", inMemFetcher, &MetaData{Arranger: "Ano"}, 1},
		{"No Match", inMemFetcher, &MetaData{Title: "Nonexistent"}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {

			results, err := test.fetcher.MetaByPattern(context.Background(), test.pattern)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(results) != test.expected {
				t.Errorf("Expected %d results, got %d", test.expected, len(results))
			}
		})
	}
}

func TestSubmit(t *testing.T) {
	inMemStore := &InMemoryStore{
		Data:     make(map[string][]byte),
		Metadata: []MetaData{},
	}

	meta := &MetaData{
		Title:    "Test Title",
		Composer: "Test Composer",
		Arranger: "Test Arranger",
	}

	content := "This is a test content."

	err := inMemStore.Submit(context.Background(), meta, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(inMemStore.Data) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(inMemStore.Data))
	}

	if string(inMemStore.Data[meta.ResourceName()]) != content {
		t.Errorf("Expected content '%s', got '%s'", content, string(inMemStore.Data[meta.ResourceName()]))
	}
}

func TestErrRetrievingContentOnFailingReader(t *testing.T) {
	inMemStore := &InMemoryStore{
		Data:     make(map[string][]byte),
		Metadata: []MetaData{},
	}

	meta := &MetaData{
		Title:    "Test Title",
		Composer: "Test Composer",
		Arranger: "Test Arranger",
	}

	err := inMemStore.Submit(context.Background(), meta, &failingReader{}) // Passing nil to simulate a failing reader
	if err == nil {
		t.Fatal("Expected an error when submitting with a nil reader, but got none")
	}

	if !errors.Is(err, ErrRetrievingContent) {
		t.Errorf("Expected error to contain '%s', got '%s'", ErrRetrievingContent.Error(), err.Error())
	}
}

func TestAppendWhenExist(t *testing.T) {
	var buffer1 bytes.Buffer
	zipWriter := zip.NewWriter(&buffer1)
	w, err := zipWriter.Create("file1.txt")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("Content1"))
	zipWriter.Close()

	var buffer2 bytes.Buffer
	zipWriter2 := zip.NewWriter(&buffer2)
	w, err = zipWriter2.Create("file2.txt")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("Content2"))
	zipWriter2.Close()

	store := NewInMemoryStore()

	meta := MetaData{
		Composer: "Unknown composer",
		Arranger: "None",
		Title:    "My song",
	}
	store.Submit(context.Background(), &meta, &buffer1)
	store.Submit(context.Background(), &meta, &buffer2)

	resourceName := meta.ResourceName()

	content := store.Data[resourceName]
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}

	if len(zipReader.File) != 2 {
		t.Fatalf("Expected 2 files got '%d'", len(zipReader.File))
	}

}

func TestProjectByName(t *testing.T) {
	inMemStore := &InMemoryStore{
		Projects: map[string]Project{
			"testproject":    {Name: "Test Project", ResourceIds: []string{"res1", "res2"}},
			"anotherproject": {Name: "Another Project", ResourceIds: []string{"res3"}},
		},
	}

	tests := []struct {
		name        string
		projectName string
		expected    int
	}{
		{"Existing Project", "test", 1},
		{"Non-existing Project", "Non-existing Project", 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results, err := inMemStore.ProjectsByName(context.Background(), test.projectName)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(results) != test.expected {
				t.Errorf("Expected %d projects, got %d", test.expected, len(results))
			}
		})
	}
}

func TestNewDemoStore(t *testing.T) {
	store := NewDemoStore()
	if len(store.Data) == 0 {
		t.Error("Expected demo store to have some data, but it is empty")
	}
	if len(store.Metadata) == 0 {
		t.Error("Expected demo store to have some metadata, but it is empty")
	}
	if len(store.Projects) == 0 {
		t.Error("Expected demo store to have some projects, but it is empty")
	}
}

func TestSubmitProject(t *testing.T) {
	inMemStore := &InMemoryStore{
		Projects: make(map[string]Project),
	}

	project := &Project{
		Name:        "Test Project",
		ResourceIds: []string{"res1", "res2"},
	}

	err := inMemStore.SubmitProject(context.Background(), project)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(inMemStore.Projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(inMemStore.Projects))
	}

	if inMemStore.Projects[project.Id()].Name != project.Name {
		t.Errorf("Expected project name '%s', got '%s'", project.Name, inMemStore.Projects[project.Id()].Name)
	}

	project.ResourceIds = append(project.ResourceIds, "res3")
	err = inMemStore.SubmitProject(context.Background(), project)
	if err != nil {
		t.Fatalf("Expected no error on updating project, got %v", err)
	}
	if len(inMemStore.Projects[project.Id()].ResourceIds) != 3 {
		t.Errorf("Expected 3 resource IDs in project, got %d", len(inMemStore.Projects[project.Id()].ResourceIds))
	}
}

func TestProjectById(t *testing.T) {
	inMemStore := &InMemoryStore{
		Projects: map[string]Project{
			"testproject": {Name: "Test Project", ResourceIds: []string{"res1", "res2"}},
		},
	}

	tests := []struct {
		name      string
		projectId string
		expected  string
	}{
		{"Existing Project", "testproject", "Test Project"},
		{"Non-existing Project", "nonexisting", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project, err := inMemStore.ProjectById(context.Background(), test.projectId)
			if err != nil && test.expected != "" {
				t.Fatalf("Unexpected error: %v", err)
			}
			if project != nil && project.Name != test.expected {
				t.Errorf("Expected project name '%s', got '%s'", test.expected, project.Name)
			} else if project == nil && test.expected != "" {
				t.Error("Expected a project but got nil")
			}
		})
	}
}

func TestMetaById(t *testing.T) {
	inMemStore := &InMemoryStore{
		Metadata: []MetaData{
			{Title: "Test Title", Composer: "Test Composer", Arranger: "Test Arranger"},
			{Title: "Another Title", Composer: "Another Composer", Arranger: "Another Arranger"},
		},
	}

	tests := []struct {
		name     string
		metaId   string
		expected string
	}{
		{"Existing Meta", "b51b44dd2b01d6553d4718c74ed4ed68", "Test Title"},
		{"Non-existing Meta", "nonexisting", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meta, err := inMemStore.MetaById(context.Background(), test.metaId)
			if err != nil && test.expected != "" {
				t.Fatalf("Unexpected error: %v", err)
			}
			if meta != nil && meta.Title != test.expected {
				t.Errorf("Expected meta title '%s', got '%s'", test.expected, meta.Title)
			} else if meta == nil && test.expected != "" {
				t.Error("Expected metadata but got nil")
			}
		})
	}
}

func TestResourceById(t *testing.T) {
	store := NewDemoStore()
	name := store.Metadata[0].ResourceName()

	reader, err := store.Resource(context.Background(), name)
	if err != nil {
		t.Fatalf("ResourceById: %s", err)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read all: %s", err)
	}
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Failed to create zip reader: %s", err)
	}

	if n := len(zipReader.File); n != 5 {
		t.Fatalf("Expected 5 files got %d", n)
	}
}

func TestResourceByIdUnknownId(t *testing.T) {
	store := NewDemoStore()
	_, err := store.Resource(context.Background(), "unknownName")
	if !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("Wanted %s got %s", ErrResourceNotFound, err)
	}
}

func TestClone(t *testing.T) {
	for i, modifier := range []func(s *InMemoryStore){
		func(s *InMemoryStore) { s.Metadata[1].Composer = "Some random guy" },
		func(s *InMemoryStore) {
			for k := range s.Projects {
				p := s.Projects[k]
				p.Name = "New name"
				s.Projects[k] = p
				break
			}
		},
		func(s *InMemoryStore) {
			for k, v := range s.Data {
				v = append(v, 0x00)
				s.Data[k] = v
				break
			}
		},
		func(s *InMemoryStore) { s.Metadata = append(s.Metadata, MetaData{}) },
		func(s *InMemoryStore) { s.Projects["new-project"] = Project{} },
		func(s *InMemoryStore) { s.Data["new-data"] = []byte{} },
	} {
		t.Run(fmt.Sprintf("Test #%d", i), func(t *testing.T) {
			store := NewDemoStore()
			clone := store.Clone()
			if !reflect.DeepEqual(store, clone) {
				t.Fatalf("Clone not equal. Original\n%+v\nClone\n%+v", store, clone)
			}

			modifier(store)
			if reflect.DeepEqual(store, clone) {
				t.Fatalf("Stores should not be equal after modification")
			}
		})
	}
}

func TestDeleteResourceFromProject(t *testing.T) {
	store := NewInMemoryStore()

	project := Project{
		Name:        "myproject",
		ResourceIds: []string{"id1", "id2", "id3"},
	}

	ctx := context.Background()
	store.SubmitProject(ctx, &project)
	if err := store.RemoveResource(ctx, "myproject", "id2"); err != nil {
		t.Fatal(err)
	}

	want := []string{"id1", "id3"}
	got := store.Projects["myproject"].ResourceIds

	if slices.Compare(got, want) != 0 {
		t.Fatalf("Wanted %v got %v", want, got)
	}
}

func TestDeleteResourceErrorOnUnknownProject(t *testing.T) {
	store := NewInMemoryStore()
	err := store.RemoveResource(context.Background(), "some-non-existent-project", "resource")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("Wanted %s got %s", ErrProjectNotFound, err)
	}
}
