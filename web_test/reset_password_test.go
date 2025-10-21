package web_test

import (
	"testing"

	"github.com/davidkleiven/caesura/testutils"
	"github.com/playwright-community/playwright-go"
)

const resetPasswordPage = "/login/reset/form"

func TestResetPassword(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		password := page.Locator("input[id=password]")
		count, err := password.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)

		retyped := page.Locator("input[id=retyped]")
		count, err = retyped.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)

		flashMsg := page.Locator("div[id=flashMessage]")
		count, err = flashMsg.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)
		flashContent, err := flashMsg.TextContent()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, flashContent, "")

		submitBtn := page.Locator("button[id=reset-password-btn]")
		count, err = submitBtn.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)

		newPassword := "new-password"
		password.Fill(newPassword)
		retyped.Fill(newPassword)

		timeout := playwright.PageExpectResponseOptions{
			Timeout: playwright.Float(1000.0),
		}
		_, err = page.ExpectResponse("**/password", func() error { return submitBtn.Click() }, timeout)
		testutils.AssertNil(t, err)

		flashContent, err = flashMsg.TextContent()
		testutils.AssertNil(t, err)
		testutils.AssertContains(t, flashContent, "Invalid JWT token")

	}, resetPasswordPage)(t)
}
