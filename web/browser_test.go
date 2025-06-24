package web_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/api"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/primitives"
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

func createTextBox(txt string) *primitives.TextBox {
	return &primitives.TextBox{
		Value:    txt,
		Position: [2]float64{100, 100},
		Font: &primitives.FormFont{
			Name: "Helvetica",
			Size: 12,
		},
	}
}

func createTwoPagePdf(w io.Writer) error {
	desc := primitives.PDF{
		Pages: map[string]*primitives.PDFPage{
			"1": {
				Content: &primitives.Content{
					TextBoxes: []*primitives.TextBox{
						createTextBox("This is page 1"),
					},
				},
			},
			"2": {
				Content: &primitives.Content{
					TextBoxes: []*primitives.TextBox{
						createTextBox("This is page 2"),
					},
				},
			},
		},
	}

	data, err := json.Marshal(desc)
	if err != nil {
		return err
	}
	return pdfapi.Create(nil, bytes.NewBuffer(data), w, nil)

}

func TestLoadPdf(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		// Ensure that Page shows 0 / 0
		currentPage, err := page.Locator("#page-num").TextContent()
		if err != nil {
			t.Error(err)
			return
		}

		if currentPage != "0" {
			t.Errorf("Expected current page to be '0', but got: %s", currentPage)
		}

		pageCount, err := page.Locator("#page-count").TextContent()
		if err != nil {
			t.Error(err)
			return
		}

		if pageCount != "0" {
			t.Errorf("Expected page count to be '0', but got: %s", pageCount)
		}

		f, err := os.CreateTemp("", "test*.pdf")
		if err != nil {
			t.Error(err)
			return
		}
		defer os.Remove(f.Name())
		if err := createTwoPagePdf(f); err != nil {
			t.Error(err)
			return
		}

		if err := page.Locator("#file-input").SetInputFiles(f.Name()); err != nil {
			t.Error(err)
			return
		}

		nextPage := page.Locator("#next-page")
		prevPage := page.Locator("#prev-page")

		for i, action := range []struct {
			f            func() error
			expectedPage string
		}{
			{func() error { return nil }, "1"},
			{func() error { return nextPage.Click() }, "2"},
			{func() error { return nextPage.Click() }, "2"}, // Should stay on page 2
			{func() error { return prevPage.Click() }, "1"},
			{func() error { return prevPage.Click() }, "1"}, // Should stay on page 1
		} {
			if err := action.f(); err != nil {
				t.Errorf("Test #%d: %s", i, err)
				return
			}

			time.Sleep(500 * time.Millisecond) // Wait for the page to update

			currentPage, err = page.Locator("#page-num").TextContent()
			if err != nil {
				t.Error(err)
				return
			}
			if currentPage != action.expectedPage {
				t.Errorf("Action #%d: Expected current page to be %s, but got: %s", i, action.expectedPage, currentPage)
			}
		}

	})(t)
}

func TestAssign(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		f, err := os.CreateTemp("", "test*.pdf")
		if err != nil {
			t.Error(err)
			return
		}
		defer os.Remove(f.Name())
		if err := createTwoPagePdf(f); err != nil {
			t.Error(err)
			return
		}

		if err := page.Locator("#file-input").SetInputFiles(f.Name()); err != nil {
			t.Error(err)
			return
		}

		waitOpts := playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(5000)}
		if _, err := page.WaitForFunction(`() => document.querySelector("#page-count").textContent === "2"`, waitOpts); err != nil {
			t.Errorf("Failed to load PDF: %s", err)
			return
		}

		assignButton := page.Locator("#assign-page")

		// Add response to alerts
		alertTriggered := false
		page.On("dialog", func(dialog playwright.Dialog) {
			alertTriggered = true
			dialog.Accept()
		})

		// Click the assign button when no group is selected (should trigger an alert)
		assignButton.Click()
		if !alertTriggered {
			t.Error("Expected alert to be triggered, but it was not.")
			return
		}

		// Trigger population of the chosen instrument field
		trumpetElement := page.Locator("li:text('Piccolo Trumpet')").First()
		if err := trumpetElement.Click(); err != nil {
			t.Error(err)
			return
		}

		if txt, err := page.Locator("#chosen-instrument").TextContent(); txt != "Piccolo Trumpet" || err != nil {
			t.Errorf("Expected chosen instrument to be 'Piccolo Trumpet', but got: %s (err: %v)", txt, err)
			return
		}

		// Test that the assignment tab is updated when clicking subsequent times
		for i, expect := range []struct {
			from string
			to   string
		}{
			{from: "1", to: "1"},
			{from: "1", to: "2"},
			{from: "1", to: "2"},
		} {
			assignButton.Click()

			// Confirm that there is an element with id "trumpet"
			trumpetElementAssignment := page.Locator("#piccolotrumpet")
			if exists, err := trumpetElementAssignment.Count(); err != nil || exists != 1 {
				t.Errorf("Click %d: Err %v number of occurences of #trumpet: %d", i, err, exists)
				return
			}

			if txt, err := page.Locator("#piccolotrumpet-from").TextContent(); err != nil || txt != expect.from {
				t.Errorf("Click %d: Expected #piccolotrumpet-from to be '%s', but got: %s (err: %v)", i, expect.to, txt, err)
				return
			}

			if txt, err := page.Locator("#piccolotrumpet-to").TextContent(); err != nil || txt != expect.to {
				t.Errorf("Click %d: Expected #piccolotrumpet-to to be '%s', but got: %s (err: %v)", i, expect.to, txt, err)
				return
			}
		}

		// Check that assignment tab is deleted when clocking it
		if err := page.Locator("#piccolotrumpet").Click(); err != nil {
			t.Error(err)
			return
		}
		if exists, err := page.Locator("#piccolotrumpet").Count(); err != nil || exists != 0 {
			t.Errorf("Expected #piccolotrumpet to be deleted, but it still exists (count: %d, err: %v)", exists, err)
			return
		}

		if err := page.Locator("#part-number").Fill("1 brass part"); err != nil {
			t.Error("Failed to fill part number field")
			return
		}

		for i, check := range []struct {
			from      string
			to        string
			alert     bool
			preAction func()
		}{
			{from: "2", to: "2", alert: false, preAction: func() {}},

			// Should trigger alert because we set the page back
			{from: "2", to: "2", alert: true, preAction: func() {
				page.Locator("#prev-page").Click()
				if _, err := page.WaitForFunction(`() => document.querySelector("#page-num").textContent === "1"`, waitOpts); err != nil {
					t.Errorf("Failed to load PDF: %s", err)
					return
				}
			}},
		} {
			check.preAction()
			alertTriggered = false
			assignButton.Click()
			if num, err := page.Locator("#piccolotrumpet1brasspart").Count(); err != nil || num != 1 {
				t.Errorf("Click #%d: Expected #piccolotrumpet1brasspart to be created, but it does not exist (count: %d, err: %v)", i, num, err)
				return
			}

			if alertTriggered != check.alert {
				t.Errorf("Click #%d: Expected alert to be %v, but it was %v", i, check.alert, alertTriggered)
				return
			}

			if txt, err := page.Locator("#piccolotrumpet1brasspart-from").TextContent(); err != nil || txt != check.from {
				t.Errorf("Click #%d: Expected #piccolotrumpet1brasspart-from to be '%s', but got: %s (err: %v)", i, check.from, txt, err)
				return
			}
			if txt, err := page.Locator("#piccolotrumpet1brasspart-to").TextContent(); err != nil || txt != check.to {
				t.Errorf("Click #%d: Expected #piccolotrumpet1brasspart-to to be '%s', but got: %s (err: %v)", i, check.to, txt, err)
				return
			}
		}
	})(t)
}
