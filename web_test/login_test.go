package web_test

import (
	"context"
	"testing"

	"github.com/davidkleiven/caesura/testutils"
	"github.com/playwright-community/playwright-go"
)

const loginPage = "/login"

func TestRegisterNewUser(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		email := page.Locator("input[id=email]")
		count, err := email.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)

		err = email.Fill("john@example.com")
		testutils.AssertNil(t, err)

		password := page.Locator("input[id=password]")
		err = password.Fill("secret-password")
		testutils.AssertNil(t, err)

		retyped := page.Locator("input[id=retyped]")
		hidden, err := retyped.IsHidden()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, hidden, true)

		// Enable the retyped field
		btn := page.Locator("#register-new-btn")
		count, err = btn.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)

		err = btn.Click()
		testutils.AssertNil(t, err)

		hidden, err = retyped.IsHidden()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, hidden, false)

		err = retyped.Fill("secret-password")
		testutils.AssertNil(t, err)

		submitBtn := page.Locator("#login-btn")
		count, err = submitBtn.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)

		timeout := playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000.0)}
		_, err = page.ExpectResponse("**/login/basic", func() error { return submitBtn.Click() }, timeout)
		testutils.AssertNil(t, err)

		_, err = store.UserByEmail(context.Background(), "john@example.com")
		testutils.AssertNil(t, err)

		flash := page.Locator("#flashMessage")
		count, err = flash.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)

		flashContent, err := flash.TextContent()
		testutils.AssertNil(t, err)
		testutils.AssertContains(t, flashContent, "success")
	}, loginPage)(t)
}

func TestForgotPassword(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		forgotPasswd := page.Locator("button[id=forgot-password-btn]")
		count, err := forgotPasswd.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)

		flashMsg := page.Locator("div[id=flashMessage]")
		count, err = flashMsg.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, count, 1)

		timeout := playwright.PageExpectResponseOptions{
			Timeout: playwright.Float(1000.0),
		}

		_, err = page.ExpectResponse("**/login/reset", func() error { return forgotPasswd.Click() }, timeout)
		testutils.AssertNil(t, err)

		flashContent, err := flashMsg.TextContent()
		testutils.AssertNil(t, err)
		testutils.AssertContains(t, flashContent, "Invalid email")

	}, loginPage)(t)
}
