package web_test

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/davidkleiven/caesura/api"
	"github.com/playwright-community/playwright-go"
)

func withBrowser(testFunc func(t *testing.T, page playwright.Page)) func(t *testing.T) {
	return func(t *testing.T) {
		pw, err := playwright.Run()
		if err != nil {
			failInCi(t, err)
			return
		}
		defer pw.Stop()

		browser, err := pw.Chromium.Launch()
		if err != nil {
			failInCi(t, err)
			return
		}
		defer browser.Close()
		page, err := browser.NewPage()
		if err != nil {
			failInCi(t, err)
			return
		}

		mux := api.Setup()
		server := httptest.NewServer(mux)
		defer server.Close()

		if _, err := page.Goto(server.URL); err != nil {
			failInCi(t, err)
			return
		}
		testFunc(t, page)
	}
}

func failInCi(t *testing.T, err error) {
	if _, inCi := os.LookupEnv("CI"); inCi {
		t.Error(err)
	} else {
		t.Skip(err)
	}
}

func TestInstrumentListIsLoaded(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		element := page.Locator("#instrument-list")
		text, err := element.TextContent()
		if err != nil {
			t.Error(err)
			return
		}

		if !strings.Contains(text, "Trumpet") {
			t.Errorf("Expepected to find 'Trumpet' in the instrument list, but got: %s", text)
		}
	})(t)
}

func TestFilterList(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		err := page.Locator("input[name='token']").Fill("flu")
		if err != nil {
			t.Error(err)
			return
		}

		// Trigger key-up
		if err := page.Locator("input[name='token']").Press("Enter"); err != nil {
			t.Error(err)
			return
		}

		response, err := page.ExpectResponse(
			"**/instruments**",
			func() error { return nil },
			playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)},
		)

		if err != nil || !response.Ok() {
			t.Error(err)
			return
		}

		element := page.Locator("#instrument-list")
		text, err := element.TextContent()
		if err != nil {
			t.Error(err)
			return
		}

		for _, instrument := range strings.Split(text, "\n") {
			if strings.ReplaceAll(instrument, " ", "") != "" && !strings.Contains(instrument, "Flute") {
				t.Errorf("Expected to find 'Flute' in the instrument list, but got: %s", text)
			}
		}
	})(t)

}

func TestFieldPopulatedOnClick(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		trumpetElement := page.Locator("li:text('Trumpet')").First()
		if err := trumpetElement.Click(); err != nil {
			t.Error(err)
			return
		}

		chosenInstrument := page.Locator("#chosen-instrument")
		text, err := chosenInstrument.TextContent()
		if err != nil {
			t.Error(err)
			return
		}
		if !strings.Contains(text, "Trumpet") {
			t.Errorf("Expected to find 'Trumpet' in the chosen instrument, but got: %s", text)
		}

	})(t)
}
