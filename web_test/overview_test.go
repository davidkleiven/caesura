package web_test

import (
	"fmt"
	"net/http"
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

		rowCount, err := page.Locator("table tbody tr").Count()
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

		rowCount, err := page.Locator("table tbody tr").Count()
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
