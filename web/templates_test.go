package web

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/pkg"
)

func TestIndex(t *testing.T) {
	index := Upload(&ScoreMetaData{})

	if !bytes.Contains(index, []byte("Caesura</div>")) {
		t.Error("Expected index to contain 'Caesura</div>'")
	}
}

func TestList(t *testing.T) {
	list := List()

	if !bytes.Contains(list, []byte("</ul>")) {
		t.Error("Expected list to contain '</ul>'")
	}
}

func TestOverview(t *testing.T) {
	overview := Overview()

	if !bytes.Contains(overview, []byte("Title")) {
		t.Error("Expected overview to contain 'Title")
	}
}

func TestResourceList(t *testing.T) {
	var buf bytes.Buffer
	ResourceList(&buf, []pkg.MetaData{
		{Title: "Test Title", Composer: "Test Composer", Arranger: "Test Arranger"},
	})

	if !bytes.Contains(buf.Bytes(), []byte("Test Title")) {
		t.Error("Expected resource list to contain 'Test Title'")
	}
}

func TestProjectSelectorModal(t *testing.T) {
	projectSelector := ProjectSelectorModal()

	if !bytes.Contains(projectSelector, []byte("Confirm")) {
		t.Error("Expected project selector modal to contain 'Confirm'")
	}
}

func TestProjectQueryInput(t *testing.T) {
	var buf bytes.Buffer
	ProjectQueryInput(&buf, "Test Query")

	if !bytes.Contains(buf.Bytes(), []byte("Test Query")) {
		t.Error("Expected project query input to contain 'Test Query'")
	}
}

func TestProjects(t *testing.T) {
	projects := Projects()

	if !bytes.Contains(projects, []byte("# pieces")) {
		t.Error("Expected projects to contain '# pieces'")
	}
}

func TestProjectList(t *testing.T) {
	var buf bytes.Buffer
	tz, err := time.LoadLocation("Europe/Stockholm")
	if err != nil {
		t.Fatalf("Failed to load timezone: %v", err)
	}

	date := time.Date(1991, 6, 6, 5, 5, 5, 0, tz)
	ProjectList(&buf, []pkg.Project{
		{Name: "Test Project", CreatedAt: date, UpdatedAt: date, ResourceIds: []string{"res1", "res2"}},
	})

	content := buf.String()

	expect := []string{
		"Test Project",
		"Thu, 06 Jun 1991 05:05:05 CEST",
	}

	for _, exp := range expect {
		if !strings.Contains(content, exp) {
			t.Errorf("Expected project list to contain '%s', but it did not", exp)
		}
	}
}

func TestProjectContent(t *testing.T) {
	var buf bytes.Buffer

	resources := []pkg.MetaData{
		{Title: "Resource 1", Composer: "Composer A", Arranger: "Arranger X"},
		{Title: "Resource 2", Composer: "Composer B", Arranger: "Arranger Y"},
	}

	project := &pkg.Project{
		Name:        "Test Project",
		ResourceIds: []string{resources[0].ResourceId()},
	}

	ProjectContent(&buf, project, resources)

	content := buf.String()

	expect := []string{
		"Test Project",
		"Resource 1",
		"Composer A",
		"Arranger X",
		"<tbody",
		"</tbody>",
	}

	for _, exp := range expect {
		if !strings.Contains(content, exp) {
			t.Errorf("Expected project content to contain '%s', but it did not", exp)
		}
	}
}
