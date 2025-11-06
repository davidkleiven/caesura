package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
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
			t.Fatalf("Error waiting for initial load: %v", err)

		}

		rowCount, err := page.Locator("table tbody tr[id^='row']").Count()
		if err != nil {
			t.Fatalf("Error counting rows: %v", err)

		}

		if rowCount != 2 {
			t.Fatalf("Expected 2 rows, got %d", rowCount)

		}
	}, overViewPage)(t)
}

func TestSearchForTitle(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Fatal(err)

		}

		searchInput := page.Locator("input[name=resource-filter]")
		if err := searchInput.Fill("arranger x"); err != nil {
			t.Fatal(err)

		}

		resp, err := page.ExpectResponse("**/overview/search**", func() error {
			return searchInput.Press("Enter")
		}, playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)})
		if err != nil {
			t.Fatal(err)

		}

		if resp.Status() != http.StatusOK {
			t.Fatalf("Expected OK response, but got %d", resp.Status())

		}

		rowCount, err := page.Locator("table tbody tr[id^='row']").Count()
		if err != nil {
			t.Fatalf("Error counting rows: %v", err)

		}

		if rowCount != 1 {
			t.Fatalf("Expected 1 row, got %d", rowCount)

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

	resp, err = page.ExpectResponse("**/projects/names**", nil, timout)
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
			t.Fatalf("Error opening project selector: %v", err)

		}

		confirmButton := page.Locator("button:has-text('Confirm')")
		resp, err := page.ExpectResponse("**/projects**", func() error { return confirmButton.Click() }, playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)})
		if err != nil {
			t.Fatalf("Error clicking confirm button: %v", err)

		}

		if resp.Status() != http.StatusBadRequest {
			t.Fatalf("Expected error response, but got OK")

		}

		// Confirm modal disappears on click
		modalContent, err := page.Locator("#project-selection-modal").TextContent()
		if err != nil {
			t.Fatalf("Error getting modal content: %v", err)

		}
		if modalContent != "" {
			t.Fatalf("Expected modal to be closed, but it still has content: %s", modalContent)

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
			t.Fatalf("Error opening project selector: %v", err)

		}

		timeout := playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)}
		existing := page.Locator("#project-selection-modal li").First()
		resp, err := page.ExpectResponse("**/project-query-input**", func() error { return existing.Click() }, timeout)
		if err != nil {
			t.Fatalf("Error clicking existing project: %v", err)

		}
		if resp.Status() != http.StatusOK {
			t.Fatalf("Expected OK response, but got %d", resp.Status())

		}

		confirmButton := page.Locator("button:has-text('Confirm')")
		resp, err = page.ExpectResponse("**/projects**", func() error { return confirmButton.Click() }, timeout)
		if err != nil {
			t.Fatalf("Error clicking confirm button: %v", err)

		}

		if resp.Status() != http.StatusOK {
			t.Fatalf("Expected OK response, but got %d", resp.Status())

		}

		flashMsg, err := page.Locator("#flashMessage").TextContent()
		if err != nil {
			t.Fatalf("Error getting flash message: %v", err)

		}

		expectedMsg := "Added 1 piece(s) to 'Demo Project 1'"
		if flashMsg != expectedMsg {
			t.Fatalf("Expected flash message to be %s, got '%s'", expectedMsg, flashMsg)

		}
	}, overViewPage)(t)
}

func expandFirstRow(page playwright.Page) (playwright.Locator, error) {
	row := page.Locator(`button[title="Show content"]`).First()

	timeout := playwright.PageExpectResponseOptions{
		Timeout: playwright.Float(1000),
	}
	resp, err := page.ExpectResponse("**/resources/*/content", func() error { return row.Click() }, timeout)

	if err != nil {
		return row, err
	}

	if !resp.Ok() {
		return row, fmt.Errorf("Response code %d", resp.Status())
	}
	return row, nil
}

func TestResourcesDisplayOnClick(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Fatal(err)

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

		downloadLink := page.Locator(`a[href^="/resources"]`).First()

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
			t.Fatal(err)

		}

		if _, err := expandFirstRow(page); err != nil {
			t.Fatal(err)
		}

		pattern := `a[href^="/resources"][href*="file="]`
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

func TestAddToResource(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Fatal(err)
		}

		pattern := `a[href^="/resources"][href*="/submit-form"]`
		addBtn := page.Locator(pattern)

		if cnt, err := addBtn.Count(); err != nil || cnt == 0 {
			t.Fatalf("Expected at least one add button got (error, num): (%s, %d)", err, cnt)
		}

		if err := addBtn.First().Click(); err != nil {
			t.Fatal(err)
		}
		timeout := playwright.LocatorWaitForOptions{Timeout: playwright.Float(1000)}
		if err := page.Locator(`div[id="upload-form"]`).WaitFor(timeout); err != nil {
			t.Fatal(err)
		}

		patternsResult := map[string]string{
			`input[name="composer"]`: "Composer A",
			`input[name="arranger"]`: "Arranger X",
			`input[name="title"]`:    "Demo Title 1",
		}

		for pattern, want := range patternsResult {
			value, err := page.Locator(pattern).InputValue()
			if err != nil {
				t.Fatal(err)
			}

			if value != want {
				t.Fatalf("Wanted %s got %s", want, value)
			}
		}

	}, overViewPage)(t)
}

func TestAddToItemNotHidden(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Fatal(err)
		}

		addItemLocator, err := page.Locator(`a[title="Add item"]`).All()

		if err != nil {
			t.Fatal(err)
		}

		for _, button := range addItemLocator {
			hidden, err := button.IsHidden()
			if err != nil {
				t.Fatal(err)
			}
			if hidden {
				t.Fatal("Add item button should not be hidden")
			}
		}

	}, overViewPage)(t)
}

func TestDownloadCheckedItems(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		if err := waitForInitialLoad(page); err != nil {
			t.Fatal(err)
		}

		checkbox := page.Locator("input[type='checkbox']")
		cnt, err := checkbox.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, cnt, 2)

		// Check the first checkbox
		err = checkbox.First().Check()
		testutils.AssertNil(t, err)

		btn := page.Locator("#distribute-btn")

		num, err := btn.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, num, 1)

		var resourceIds []string
		requestInspector := func(request playwright.Request) {
			if strings.Contains(request.URL(), "/resources/parts") && request.Method() == "POST" {
				body, err := request.PostData()
				testutils.AssertNil(t, err)

				values, err := url.ParseQuery(body)
				testutils.AssertNil(t, err)

				formRecourceIds := values["resourceId"]
				resourceIds = append(resourceIds, formRecourceIds...)
			}
		}
		page.On("request", requestInspector)

		timeout := playwright.PageExpectResponseOptions{
			Timeout: playwright.Float(1000),
		}
		_, err = page.ExpectResponse("**/resources/parts", func() error {
			return btn.Click()
		}, timeout)
		testutils.AssertNil(t, err)

		want := []string{"demotitle1_composera_arrangerx"}
		testutils.AssertEqual(t, len(resourceIds), len(want))

		for i, v := range resourceIds {
			testutils.AssertEqual(t, v, want[i])
		}
	}, overViewPage)(t)
}
