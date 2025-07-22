package web_test

import (
	"testing"

	"github.com/playwright-community/playwright-go"
)

const projectPage = "/projects"

func waitForProjectPageLoad(page playwright.Page) error {
	locator := page.Locator("#project-list td:has-text('Demo Project 1')")

	waitForOpts := playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(1000),
	}
	return locator.WaitFor(waitForOpts)
}

func TestProjectClick(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForProjectPageLoad(page); err != nil {
			t.Error(err)
			return
		}

		clickableRow := page.Locator("#demoproject1")

		timeout := playwright.PageExpectResponseOptions{
			Timeout: playwright.Float(1000),
		}
		resp, err := page.ExpectResponse("**/projects/demoproject1**", func() error { return clickableRow.Click() }, timeout)
		if err != nil {
			t.Error(err)
			return
		}
		if resp.Status() != 200 {
			t.Errorf("Expected status code 200, got %d", resp.Status())
			return
		}

		// Expect songs of demoproject1 to be present
		songLocator := page.Locator("#piece-list td:has-text('Composer A')")
		if err := songLocator.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(1000)}); err != nil {
			t.Error(err)
			return
		}
	}, projectPage)(t)
}

func fillProjectQueryInput(page playwright.Page, query string) func() error {
	return func() error {
		searchInput := page.Locator("input[name='projectQuery']")
		if err := searchInput.Fill(query); err != nil {
			return err
		}
		return searchInput.Press("Enter")
	}
}

func TestProjectSearch(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForProjectPageLoad(page); err != nil {
			t.Error(err)
			return
		}

		timeout := playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)}
		for _, check := range []struct {
			query         string
			expectedCount int
		}{
			{
				query:         "demo",
				expectedCount: 1,
			},
			{
				query:         "non-existent",
				expectedCount: 0,
			},
		} {

			resp, err := page.ExpectResponse("**/projects/info**", fillProjectQueryInput(page, check.query), timeout)

			if err != nil {
				t.Error(err)
				return
			}

			if !resp.Ok() {
				t.Errorf("Expected response to be OK, got %d", resp.Status())
				return
			}

			projectLocator, err := page.Locator("#project-list td:has-text('Demo Project 1')").Count()
			if err != nil {
				t.Error(err)
				return
			}
			if projectLocator != check.expectedCount {
				t.Errorf("Expected %d project with 'Demo Project 1', found %d", check.expectedCount, projectLocator)
				return
			}
		}

	}, projectPage)(t)
}

func TestAddToItemIsHidden(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForProjectPageLoad(page); err != nil {
			t.Fatal(err)
		}

		hidden, err := page.Locator(`a[title="Add item"]`).IsHidden()
		if err != nil {
			t.Fatal(err)
		}
		if !hidden {
			t.Fatal("Add item button should be hidden")
		}
	}, projectPage)(t)
}
