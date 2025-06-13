package web_test

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davidkleiven/caesura/api"
	"github.com/davidkleiven/caesura/utils"
	"github.com/playwright-community/playwright-go"
)

func runBrowserTest(testFunc func(page playwright.Page) error) error {
	pw := utils.Must(playwright.Run())
	defer pw.Stop()

	browser := utils.Must(pw.Chromium.Launch())
	defer browser.Close()
	page := utils.Must(browser.NewPage())

	mux := api.Setup()
	server := httptest.NewServer(mux)
	defer server.Close()

	utils.Must(page.Goto(server.URL))
	return testFunc(page)
}

func TestInstrumentListIsLoaded(t *testing.T) {
	err := runBrowserTest(func(page playwright.Page) error {
		element := page.Locator("#instrument-list")
		text, err := element.TextContent()
		if err != nil {
			return err
		}

		if !strings.Contains(text, "Trumpet") {
			return fmt.Errorf("Expepected to find 'Trumpet' in the instrument list, but got: %s", text)
		}
		return nil
	})

	if err != nil {
		t.Error(err)
	}
}
