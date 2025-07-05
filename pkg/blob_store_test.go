package pkg

import "testing"

func TestProjectId(t *testing.T) {
	project := &Project{
		Name: "Test Project",
	}

	expectedId := "testproject"
	if project.Id() != expectedId {
		t.Errorf("Expected project ID '%s', got '%s'", expectedId, project.Id())
	}
}

func TestMergeProjects(t *testing.T) {
	project1 := &Project{
		Name:        "Project A",
		ResourceIds: []string{"res1", "res2"},
	}
	project2 := &Project{
		Name:        "Project B",
		ResourceIds: []string{"res2", "res3"},
	}

	project1.Merge(project2)

	expectedResourceIds := []string{"res1", "res2", "res3"}
	if len(project1.ResourceIds) != len(expectedResourceIds) {
		t.Errorf("Expected %d resource IDs, got %d", len(expectedResourceIds), len(project1.ResourceIds))
	}
}
