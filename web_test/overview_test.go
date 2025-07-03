package web_test

import (
	"fmt"
	"testing"

	"github.com/playwright-community/playwright-go"
)

const overViewPage = "/overview"

func waitForInitialLoad(page playwright.Page) error {
	response, err := page.ExpectResponse(
		"**/overview/search**",
		func() error { return nil },
		playwright.PageExpectResponseOptions{Timeout: playwright.Float(4000)},
	)

	if err != nil {
		return err
	}

	if !response.Ok() {
		return fmt.Errorf("Expected response to be OK, but got: %d", response.Status())
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

		if err := searchInput.Press("Enter"); err != nil {
			t.Error(err)
			return
		}

		if err := waitForInitialLoad(page); err != nil {
			t.Error(err)
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
