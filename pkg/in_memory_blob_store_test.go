package pkg

import (
	"context"
	"errors"
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
