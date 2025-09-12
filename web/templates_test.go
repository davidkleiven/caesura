package web

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
)

func TestUpload(t *testing.T) {
	index := Upload(&ScoreMetaData{}, "en")

	if !bytes.Contains(index, []byte("Caesura</div>")) {
		t.Fatal("Expected index to contain 'Caesura</div>'")
	}
}

func TestList(t *testing.T) {
	list := List()

	if !bytes.Contains(list, []byte("</ul>")) {
		t.Fatal("Expected list to contain '</ul>'")
	}
}

func TestOverview(t *testing.T) {
	overview := Overview("en")

	if !bytes.Contains(overview, []byte("Title")) {
		t.Fatal("Expected overview to contain 'Title")
	}
}

func TestResourceList(t *testing.T) {
	var buf bytes.Buffer
	ResourceList(&buf, []pkg.MetaData{
		{Title: "Test Title", Composer: "Test Composer", Arranger: "Test Arranger"},
	})

	if !bytes.Contains(buf.Bytes(), []byte("Test Title")) {
		t.Fatal("Expected resource list to contain 'Test Title'")
	}
}

func TestProjectSelectorModal(t *testing.T) {
	projectSelector := ProjectSelectorModal("en")

	if !bytes.Contains(projectSelector, []byte("Confirm")) {
		t.Fatal("Expected project selector modal to contain 'Confirm'")
	}
}

func TestProjectQueryInput(t *testing.T) {
	var buf bytes.Buffer
	ProjectQueryInput(&buf, "en", "Test Query")

	if !bytes.Contains(buf.Bytes(), []byte("Test Query")) {
		t.Fatal("Expected project query input to contain 'Test Query'")
	}
}

func TestProjects(t *testing.T) {
	projects := Projects("en")

	if !bytes.Contains(projects, []byte("# pieces")) {
		t.Fatal("Expected projects to contain '# pieces'")
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
			t.Fatalf("Expected project list to contain '%s', but it did not", exp)
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

	ProjectContent(&buf, project, resources, "en")

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
			t.Fatalf("Expected project content to contain '%s', but it did not", exp)
		}
	}
}

func TestResourceContent(t *testing.T) {
	var buf bytes.Buffer
	data := ResourceContentData{
		ResourceId: "resource-id",
		Filenames:  []string{"file.pdf", "file2.pdf"},
	}

	ResourceContent(&buf, &data)
	testutils.AssertContains(t, buf.String(), "resource-id", "file.pdf", "file2.pdf")
}

func TestOrganizations(t *testing.T) {
	content := Organizations("en")
	testutils.AssertContains(t, string(content), "</body>")
}

func TestOrganizationsList(t *testing.T) {
	var buf bytes.Buffer
	organizations := []pkg.Organization{
		{Name: "Org1"}, {Name: "Org2"},
	}

	WriteOrganizationHTML(&buf, organizations)
	testutils.AssertContains(t, buf.String(), "Org1", "Org2")
}

func TestIndex(t *testing.T) {
	index := string(Index("en"))
	testutils.AssertContains(t, index, "</body>")
}

func TestPeopleHtml(t *testing.T) {
	var buf bytes.Buffer
	WritePeopleHTML(&buf, "en")
	testutils.AssertContains(t, buf.String(), "</body>")
}

func TestWriteUserList(t *testing.T) {
	var buf bytes.Buffer
	orgId := "0000-0000-org-id"
	users := []pkg.UserInfo{
		{
			Name:  "Peter",
			Roles: map[string]pkg.RoleKind{orgId: 0},
		},
		{
			Name:  "John",
			Roles: map[string]pkg.RoleKind{orgId: 1},
		},
		{
			Name:  "Susan",
			Roles: map[string]pkg.RoleKind{orgId: 2},
		},
	}

	WriteUserList(&buf, users, orgId, []string{"opt A", "opt B"})
	testutils.AssertContains(t, buf.String(), "Peter", "John", "Susan")
}

func TestWriteStringAsOptions(t *testing.T) {
	opts := []string{"opt A", "opt B"}
	var buf bytes.Buffer
	WriteStringAsOptions(&buf, opts)
	testutils.AssertContains(t, buf.String(), "opt A", "opt B")
}

func TestSignIn(t *testing.T) {
	html := SignIn("en")
	testutils.AssertContains(t, html, "Sign in")
}

func TestSignedIn(t *testing.T) {
	testutils.AssertEqual(t, SignedIn("en"), "Signed in")
}

func TestNoOrganization(t *testing.T) {
	testutils.AssertEqual(t, NoOrganization("en"), "No organization")
}
