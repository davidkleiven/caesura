package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
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
			t.Fatal(err)

		}

		clickableRow := page.Locator("#demoproject1")

		timeout := playwright.PageExpectResponseOptions{
			Timeout: playwright.Float(1000),
		}
		resp, err := page.ExpectResponse("**/projects/demoproject1**", func() error { return clickableRow.Click() }, timeout)
		if err != nil {
			t.Fatal(err)

		}
		if resp.Status() != 200 {
			t.Fatalf("Expected status code 200, got %d", resp.Status())

		}

		// Expect songs of demoproject1 to be present
		songLocator := page.Locator("#piece-list td:has-text('Composer A')")
		if err := songLocator.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(1000)}); err != nil {
			t.Fatal(err)

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
			t.Fatal(err)

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
				t.Fatal(err)

			}

			if !resp.Ok() {
				t.Fatalf("Expected response to be OK, got %d", resp.Status())

			}

			projectLocator, err := page.Locator("#project-list td:has-text('Demo Project 1')").Count()
			if err != nil {
				t.Fatal(err)

			}
			if projectLocator != check.expectedCount {
				t.Fatalf("Expected %d project with 'Demo Project 1', found %d", check.expectedCount, projectLocator)

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

func TestRemoveFromProject(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForProjectPageLoad(page); err != nil {
			t.Fatal(err)
		}

		project1 := page.Locator(`tr[hx-get^="/projects"]`).First()

		timeout := playwright.PageExpectResponseOptions{
			Timeout: playwright.Float(1000),
		}
		_, err := page.ExpectResponse("**/projects/**", func() error { return project1.Click() }, timeout)
		if err != nil {
			t.Fatal(err)
		}

		matcher := regexp.MustCompile(`^.*/projects/[^/]+$`)

		numClicked := 0
		for i := range 2 {

			deleteBtn := page.Locator(`button[title="Remove from project"]`)
			cnt, err := deleteBtn.Count()
			if err != nil {
				t.Fatal(err)
			}

			if cnt == 0 {
				break
			}

			btn := deleteBtn.First()
			_, err = page.ExpectResponse(matcher, func() error { return btn.Click() }, timeout)
			numClicked = i + 1

			if err != nil {
				t.Fatal(err)
			}
		}

		if numClicked != 2 {
			t.Fatalf("Expected 2 clicks got %d", numClicked)
		}
	}, projectPage)(t)
}

func TestDistributeProject(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForProjectPageLoad(page); err != nil {
			t.Fatal(err)
		}

		project1 := page.Locator(`tr[hx-get^="/projects"]`).First()

		timeout := playwright.PageExpectResponseOptions{
			Timeout: playwright.Float(1000),
		}
		_, err := page.ExpectResponse("**/projects/**", func() error { return project1.Click() }, timeout)
		testutils.AssertNil(t, err)

		btn := page.Locator("#distribute-btn")
		num, err := btn.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, num, 1)

		var resourceIds []string
		requestInspector := func(request playwright.Request) {
			if strings.Contains(request.URL(), "/resources/email") && request.Method() == "POST" {
				body, err := request.PostData()
				testutils.AssertNil(t, err)

				values, err := url.ParseQuery(body)
				testutils.AssertNil(t, err)

				formRecourceIds := values["resourceId"]
				resourceIds = append(resourceIds, formRecourceIds...)
			}
		}
		page.On("request", requestInspector)

		_, err = page.ExpectResponse("**/resources/email", func() error {
			return btn.Click()
		}, timeout)
		testutils.AssertNil(t, err)

		want := []string{"demotitle1_composera_arrangerx", "demotitle2_composerb_arrangery"}
		testutils.AssertEqual(t, len(resourceIds), len(want))

		for i, v := range resourceIds {
			testutils.AssertEqual(t, v, want[i])
		}
	}, projectPage)(t)
}
