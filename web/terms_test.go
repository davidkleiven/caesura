package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
)

func TestTermsModalInIndex(t *testing.T) {
	// Test that the index template contains the terms modal
	language := "en"
	html := Index(language)

	// Check that the modal is present in the HTML
	expectedModalId := "id=\"termsModal\""
	if !strings.Contains(string(html), expectedModalId) {
		t.Errorf("Expected index template to contain terms modal with %s", expectedModalId)
	}

	// Check that the showTermsModal function is present
	expectedFunction := "showTermsModal()"
	if !strings.Contains(string(html), expectedFunction) {
		t.Errorf("Expected index template to contain %s function", expectedFunction)
	}
}

func TestLoadNorwegian(t *testing.T) {
	lang := "no"
	var buf bytes.Buffer
	TermsAndConditionsContent(&buf, lang)
	testutils.AssertContains(t, buf.String(), "Vilkår")
}

func TestLoadEnglish(t *testing.T) {
	var bufEn bytes.Buffer
	var bufUnknownLang bytes.Buffer
	TermsAndConditionsContent(&bufEn, "en")
	TermsAndConditionsContent(&bufUnknownLang, "some-unkown-language")
	testutils.AssertEqual(t, bufEn.String(), bufUnknownLang.String())
}
