package web_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/playwright-community/playwright-go"
)

const overViewPage = "/overview"

func waitForInitialLoad(page playwright.Page) error {
	tableContent := page.Locator("table tbody tr").First()
	waitForOpts := playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(1000),
	}
	if err := tableContent.WaitFor(waitForOpts); err != nil {
		return fmt.Errorf("error waiting for table content: %w", err)
	}
	return nil
}

func TestInitialLoadHasTwoItems(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Errorf("Error waiting for initial load: %v", err)
			return
		}

		rowCount, err := page.Locator("table tbody tr[id^='row']").Count()
		if err != nil {
			t.Errorf("Error counting rows: %v", err)
			return
		}

		if rowCount != 2 {
			t.Errorf("Expected 2 rows, got %d", rowCount)
			return
		}
	}, overViewPage)(t)
}

func TestSearchForTitle(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Error(err)
			return
		}

		searchInput := page.Locator("input[name=resource-filter]")
		if err := searchInput.Fill("arranger x"); err != nil {
			t.Error(err)
			return
		}

		resp, err := page.ExpectResponse("**/overview/search**", func() error {
			return searchInput.Press("Enter")
		}, playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)})
		if err != nil {
			t.Error(err)
			return
		}

		if resp.Status() != http.StatusOK {
			t.Errorf("Expected OK response, but got %d", resp.Status())
			return
		}

		rowCount, err := page.Locator("table tbody tr[id^='row']").Count()
		if err != nil {
			t.Errorf("Error counting rows: %v", err)
			return
		}

		if rowCount != 1 {
			t.Errorf("Expected 1 row, got %d", rowCount)
			return
		}

	}, overViewPage)(t)
}

func openProjectSelectorPage(page playwright.Page, preClick func(playwright.Page) error) error {
	if err := waitForInitialLoad(page); err != nil {
		return err
	}

	if preClick != nil {
		if err := preClick(page); err != nil {
			return fmt.Errorf("preClick failed: %w", err)
		}
	}

	timout := playwright.PageExpectResponseOptions{Timeout: playwright.Float(4000)}
	addButton := page.Locator("button:has-text('Add to Project')")
	resp, err := page.ExpectResponse("**/overview/project-selector**", func() error { return addButton.Click() }, timout)

	if err != nil {
		return err
	}

	if !resp.Ok() {
		return err
	}

	resp, err = page.ExpectResponse("**/search-projects**", nil, timout)
	if err != nil {
		return err
	}
	if !resp.Ok() {
		return err
	}

	numCheckBoxes, err := page.Locator("#project-selection-modal li").Count()
	if err != nil {
		return err
	}
	if numCheckBoxes != 1 {
		return fmt.Errorf("Expected 1 project checkbo (e.g. projects), got %d", numCheckBoxes)
	}
	return nil
}

func TestAddToProjectNoProjectName(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := openProjectSelectorPage(page, nil); err != nil {
			t.Errorf("Error opening project selector: %v", err)
			return
		}

		confirmButton := page.Locator("button:has-text('Confirm')")
		resp, err := page.ExpectResponse("**/add-to-project**", func() error { return confirmButton.Click() }, playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)})
		if err != nil {
			t.Errorf("Error clicking confirm button: %v", err)
			return
		}

		if resp.Status() != http.StatusBadRequest {
			t.Errorf("Expected error response, but got OK")
			return
		}

		// Confirm modal disappears on click
		modalContent, err := page.Locator("#project-selection-modal").TextContent()
		if err != nil {
			t.Errorf("Error getting modal content: %v", err)
			return
		}
		if modalContent != "" {
			t.Errorf("Expected modal to be closed, but it still has content: %s", modalContent)
			return
		}
	}, overViewPage)(t)
}

func selectFirstPiece(page playwright.Page) error {
	pieceCheckbox := page.Locator("#piece-list input[type=checkbox]").First()
	if err := pieceCheckbox.Check(); err != nil {
		return fmt.Errorf("Error checking piece checkbox: %w", err)
	}
	return nil
}

func TestAddToExistingProject(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := openProjectSelectorPage(page, selectFirstPiece); err != nil {
			t.Errorf("Error opening project selector: %v", err)
			return
		}

		timeout := playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)}
		existing := page.Locator("#project-selection-modal li").First()
		resp, err := page.ExpectResponse("**/project-query-input**", func() error { return existing.Click() }, timeout)
		if err != nil {
			t.Errorf("Error clicking existing project: %v", err)
			return
		}
		if resp.Status() != http.StatusOK {
			t.Errorf("Expected OK response, but got %d", resp.Status())
			return
		}

		confirmButton := page.Locator("button:has-text('Confirm')")
		resp, err = page.ExpectResponse("**/add-to-project**", func() error { return confirmButton.Click() }, timeout)
		if err != nil {
			t.Errorf("Error clicking confirm button: %v", err)
			return
		}

		if resp.Status() != http.StatusOK {
			t.Errorf("Expected OK response, but got %d", resp.Status())
			return
		}

		flashMsg, err := page.Locator("#flash-message").TextContent()
		if err != nil {
			t.Errorf("Error getting flash message: %v", err)
			return
		}

		expectedMsg := "Added 1 piece(s) to 'Demo Project 1'"
		if flashMsg != expectedMsg {
			t.Errorf("Expected flash message to be %s, got '%s'", expectedMsg, flashMsg)
			return
		}
	}, overViewPage)(t)
}

func expandFirstRow(page playwright.Page) (playwright.Locator, error) {
	row := page.Locator("table tbody tr[id^='row']").First()

	timeout := playwright.PageExpectResponseOptions{
		Timeout: playwright.Float(1000),
	}
	resp, err := page.ExpectResponse("**/content/**", func() error { return row.Click() }, timeout)

	if err != nil || !resp.Ok() {
		return row, fmt.Errorf("Response code: %s. (%w)", resp.StatusText(), err)
	}
	return row, nil
}

func TestResourcesDisplayOnClick(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Error(err)
			return
		}

		expandable := page.Locator("table tbody tr[id^='expand']")
		num, err := expandable.Count()
		if err != nil || num != 2 {
			t.Fatalf("Expected (num, err) = (2, nil) got (%d, %s)", num, expandable)
		}

		for i := range num {
			if hidden, err := expandable.Nth(i).IsHidden(); !hidden || err != nil {
				t.Fatalf("Expected all expandable items to initially be hidden (%v, %s)", hidden, err)
			}
		}

		row, err := expandFirstRow(page)
		if err != nil {
			t.Fatal(err)
		}

		// First should not be hidden
		if hidden, err := expandable.Nth(0).IsHidden(); hidden || err != nil {
			t.Fatalf("First item should not be hidden got (hidden, err): (%v, %s)", hidden, err)
		}

		if hidden, err := expandable.Nth(1).IsHidden(); !hidden || err != nil {
			t.Fatalf("Second item should be hidden got (hidden, err): (%v, %s)", hidden, err)
		}

		expCtn, err := expandable.First().TextContent()
		if err != nil {
			t.Fatalf("Could not received text content from expandable: %s", err)
		}
		for _, token := range []string{"Part1.pdf", "Part2.pdf", "Part3.pdf", "Part4.pdf"} {
			if !strings.Contains(expCtn, token) {
				t.Fatalf("Expected %s to be part of %s", token, expCtn)
			}
		}

		// Second click should hide the expandable item
		row.Click()
		err = expandable.First().WaitFor(
			playwright.LocatorWaitForOptions{
				State:   playwright.WaitForSelectorStateHidden,
				Timeout: playwright.Float(1000),
			},
		)

		if err != nil {
			t.Fatalf("Wait for object getting hidden: %s", err)
		}

	}, overViewPage)(t)
}

func TestDownloadZip(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Fatal(err)
		}

		downloadLink := page.Locator(`a[href^="/resource"]`).First()

		timeout := playwright.PageExpectDownloadOptions{Timeout: playwright.Float(1000.0)}
		download, err := page.ExpectDownload(func() error { return downloadLink.Click() }, timeout)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := download.Path(); err != nil {
			t.Fatal(err)
		}

		filename := download.SuggestedFilename()
		if !strings.HasSuffix(filename, "zip") {
			t.Fatalf("Expected path to have suffix 'zip' but got %s", filename)
		}
	}, overViewPage)(t)
}

func TestDownloadSinglePart(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Error(err)
			return
		}

		if _, err := expandFirstRow(page); err != nil {
			t.Fatal(err)
		}

		pattern := `a[href^="/resource"][href*="file="]`
		downloadParts := page.Locator(pattern)

		if cnt, err := downloadParts.Count(); err != nil || cnt == 0 {
			t.Fatalf("Expected at least one item to match %s got zero with error: %s", pattern, err)
		}
		downloadPart := downloadParts.First()

		timeout := playwright.PageExpectDownloadOptions{Timeout: playwright.Float(1000.0)}
		download, err := page.ExpectDownload(func() error { return downloadPart.Click() }, timeout)

		if err != nil {
			t.Fatal(err)
		}

		if _, err := download.Path(); err != nil {
			t.Fatal(err)
		}

		filename := download.SuggestedFilename()
		if !strings.HasSuffix(filename, "pdf") {
			t.Fatalf("Expected file to have suffix 'pdf' got %s", filename)
		}
	}, overViewPage)(t)
}
