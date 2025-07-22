package web_test

import (
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/api"
	"github.com/davidkleiven/caesura/pkg"
	"github.com/playwright-community/playwright-go"
)

var (
	server *httptest.Server
	page   playwright.Page
	store  *pkg.InMemoryStore = pkg.NewDemoStore()
)

func TestMain(m *testing.M) {
	pw, err := playwright.Run()
	if err != nil {
		fmt.Print(err)
		os.Exit(failureReturnCode())
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch()
	if err != nil {
		fmt.Print(err)
		os.Exit(failureReturnCode())
	}
	defer browser.Close()
	fmt.Printf("Browser launched: version=%s name=%s connected=%v\n", browser.Version(), browser.BrowserType().Name(), browser.IsConnected())

	page, err = browser.NewPage()
	if err != nil {
		fmt.Print(err)
		os.Exit(failureReturnCode())
	}

	config := pkg.NewDefaultConfig()
	mux := api.Setup(store, config)
	server = httptest.NewServer(mux)
	defer server.Close()
	fmt.Printf("Test server started. url=%s\n", server.URL)
	rcode := m.Run()
	os.Exit(rcode)
}

func withBrowser(testFunc func(t *testing.T, page playwright.Page), path string) func(t *testing.T) {
	return func(t *testing.T) {
		initialStore := store.Clone()
		defer func() {
			store.Data = initialStore.Data
			store.Metadata = initialStore.Metadata
			store.Projects = initialStore.Projects
		}()

		if _, err := page.Goto(server.URL + path); err != nil {
			t.Fatal(err)
		}
		testFunc(t, page)
	}
}

func failureReturnCode() int {
	if _, inCi := os.LookupEnv("CI"); inCi {
		return 1
	}
	return 0
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
	}, "/")(t)
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
	}, "/")(t)

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

	}, "/")(t)
}

func loadPdf(page playwright.Page, t *testing.T) func() {
	f, err := os.CreateTemp("", "test*.pdf")
	if err != nil {
		t.Error(err)
		return func() {}
	}
	removeFile := func() { os.Remove(f.Name()) }
	if err := pkg.CreateNPagePdf(f, 2); err != nil {
		t.Error(err)
		return removeFile
	}

	if err := page.Locator("#file-input").SetInputFiles(f.Name()); err != nil {
		t.Error(err)
		return removeFile
	}

	waitOpts := playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(5000)}
	if _, err := page.WaitForFunction(`() => document.querySelector("#page-count").textContent === "2"`, waitOpts); err != nil {
		t.Errorf("Failed to load PDF: %s", err)
		return removeFile
	}
	return removeFile
}

func assignPage(page playwright.Page, t *testing.T) {
	score := page.Locator("li:text('Score')").First()
	if err := score.Click(); err != nil {
		t.Error(err)
		return
	}

	assignButton := page.Locator("#assign-page")
	assignButton.Click()

	// Now the page should be set to 2
	pageNum := page.Locator("#page-num")
	if currentPage, err := pageNum.TextContent(); err != nil || currentPage != "2" {
		t.Errorf("Expected current page to be '2', but got: %s (err: %v)", currentPage, err)
		return
	}
}

func populateMetaData(page playwright.Page, t *testing.T) {
	titleInput := page.Locator("#title-input")
	if err := titleInput.Fill("Brandenburg Concerto No. 3"); err != nil {
		t.Error(err)
		return
	}

	composerInput := page.Locator("#composer-input")
	if err := composerInput.Fill("Johann Sebastian Bach"); err != nil {
		t.Error(err)
		return
	}

	arrangerInput := page.Locator("#arranger-input")
	if err := arrangerInput.Fill("Unknown"); err != nil {
		t.Error(err)
		return
	}
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

		deletePdf := loadPdf(page, t)
		defer deletePdf()

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

	}, "/")(t)
}

func TestAssign(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		cancel := loadPdf(page, t)
		defer cancel()

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

		// Enter delete mode
		if err := page.Locator("#delete-on-click").Check(); err != nil {
			t.Error("Failed to check delete mode checkbox")
			return
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

		waitOpts := playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(5000)}

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
	}, "/")(t)
}

func TestJumpToAssignedPage(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		deletePdf := loadPdf(page, t)
		defer deletePdf()

		score := page.Locator("li:text('Score')").First()
		if err := score.Click(); err != nil {
			t.Error(err)
			return
		}
		assignPage(page, t)
		pageNum := page.Locator("#page-num")

		if err := page.Locator("#score").Click(); err != nil {
			t.Error(err)
			return
		}

		// Now the page should be set to 1
		if currentPage, err := pageNum.TextContent(); err != nil || currentPage != "1" {
			t.Errorf("Expected current page to be '1', but got: %s (err: %v)", currentPage, err)
			return
		}
	}, "/")(t)
}

func TestSubmit(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		deletePdf := loadPdf(page, t)
		defer deletePdf()
		assignPage(page, t)
		populateMetaData(page, t)

		if err := page.Locator("#submit-btn").Click(); err != nil {
			t.Error(err)
			return
		}

		waitOpts := playwright.PageExpectResponseOptions{Timeout: playwright.Float(5000)}
		resp, err := page.ExpectResponse("**/resources**", func() error { return nil }, waitOpts)
		if err != nil {
			t.Errorf("Failed to submit form: %s", err)
			return
		}

		if !resp.Ok() {
			t.Errorf("Expected response to be OK, but got status: %d", resp.Status())
			return
		}

	}, "/")(t)
}
