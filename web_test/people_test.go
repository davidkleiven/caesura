package web_test

import (
	"slices"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
	"github.com/playwright-community/playwright-go"
)

const peoplePage = "/people"

func TestPeoplePage(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		users := page.Locator(`td:text("Susan")`)
		locTimeout := playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(1000),
		}
		testutils.AssertNil(t, users.WaitFor(locTimeout))

		t.Run("test register recipent", func(t *testing.T) {
			emailField := page.Locator("input[name=email]")
			testutils.AssertNil(t, emailField.Fill("peter@mail.com"))

			nameField := page.Locator("input[name=name]")
			testutils.AssertNil(t, nameField.Fill("Peter"))

			regBtn := page.Locator("#register-recipent-btn")
			testutils.AssertNil(t, regBtn.Click())

			peter := page.Locator(`td:text("peter@mail")`)
			testutils.AssertNil(t, peter.WaitFor(locTimeout))
			testutils.AssertEqual(t, len(store.Users), 3)

			var group string
			for _, g := range store.Users[2].Groups {
				group = g[0]
				break
			}

			testutils.AssertEqual(t, group, "Alto")
		})

		t.Run("test make susan soprano", func(t *testing.T) {
			orgId := "cccc13f9-ddd5-489e-bd77-3b935b457f71"
			userId := store.Users[1].Id
			groupSelector := page.Locator("#group-selector-" + userId)
			_, err := groupSelector.SelectOption(playwright.SelectOptionValues{Values: &[]string{"Soprano"}})
			testutils.AssertNil(t, err)

			groupOverview := page.Locator(`td:text("Tenor Bass Soprano")`)
			testutils.AssertNil(t, groupOverview.WaitFor(locTimeout))
			testutils.AssertEqual(t, slices.Contains(store.Users[1].Groups[orgId], "Soprano"), true)
		})
	}, peoplePage)(t)
}
