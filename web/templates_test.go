package web

import (
	"bytes"
	"testing"

	"github.com/davidkleiven/caesura/pkg"
)

func TestIndex(t *testing.T) {
	index := Index()

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
