package web_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
	"github.com/playwright-community/playwright-go"
)

const peoplePage = "/people"

var (
	locTimeout       = playwright.LocatorWaitForOptions{Timeout: playwright.Float(1000)}
	erTimeout        = playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)}
	locSelectTimeout = playwright.LocatorSelectOptionOptions{Timeout: playwright.Float(1000)}
)

func waitForPeopleInitLoad(t *testing.T, page playwright.Page) {
	users := page.Locator(`td:text("Susan")`)
	testutils.AssertNil(t, users.WaitFor(locTimeout))
}

func TestRegisterRecipent(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		waitForPeopleInitLoad(t, page)
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
	}, peoplePage)(t)
}

func TestMakeSusanSoprano(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		waitForPeopleInitLoad(t, page)
		userId := store.Users[0].Id
		groupSelector := page.Locator("#group-selector-" + userId)
		cnt, err := groupSelector.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, cnt, 1)

		_, err = page.ExpectResponse("**/organizations/users", func() error {
			opts := []string{"Soprano"}
			chosen, err1 := groupSelector.SelectOption(playwright.SelectOptionValues{Values: &opts}, locSelectTimeout)
			if slices.Compare(chosen, opts) != 0 {
				return fmt.Errorf("Wanted %v got %v", opts, chosen)
			}
			return err1
		}, erTimeout)
		testutils.AssertNil(t, err)

		tenoerBassSop := page.Locator(`td:text("Alto Soprano")`)
		num, err := tenoerBassSop.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, num, 1)
		testutils.AssertEqual(t, slices.Contains(store.Users[0].Groups[orgId], "Soprano"), true)
	}, peoplePage)(t)
}

func TestRemoveGroup(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		waitForPeopleInitLoad(t, page)
		userId := store.Users[0]
		btnId := "#remove-group-" + userId.Id + "-Alto-btn"

		btn := page.Locator(btnId)
		cnt, err := btn.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, cnt, 1)

		tdWithAlto := page.Locator(`td:has-text("Alto")`)
		numtdWithAlto, err := tdWithAlto.Count()
		testutils.AssertNil(t, err)
		if numtdWithAlto == 0 {
			t.Fatalf("Wanted at least one element with Alto")
		}

		_, err = page.ExpectResponse("**/organizations/users", func() error {
			return btn.Click()
		}, erTimeout)
		testutils.AssertNil(t, err)

		afterNumWithAlto, err := tdWithAlto.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, afterNumWithAlto, numtdWithAlto-1)
		testutils.AssertEqual(t, slices.Contains(store.Users[0].Groups[orgId], "Alto"), false)

		flashMsg := page.Locator("#flashMessage")
		count, err := flashMsg.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, 1, count)

		flashContent, err := flashMsg.TextContent()
		testutils.AssertNil(t, err)
		testutils.AssertContains(t, flashContent, "group")
	}, peoplePage)(t)
}

func TestChangeRole(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		waitForPeopleInitLoad(t, page)
		user := store.Users[0]
		selectorLoc := page.Locator("#role-select-" + user.Id)
		cnt, err := selectorLoc.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, cnt, 1)

		_, err = page.ExpectResponse("**/organizations/users/*/role", func() error {
			opts := []string{"0"}
			chosen, err1 := selectorLoc.SelectOption(playwright.SelectOptionValues{Values: &opts}, locSelectTimeout)
			if slices.Compare(chosen, opts) != 0 {
				return fmt.Errorf("Expected %v got %v", opts, chosen)
			}
			return err1
		}, erTimeout)
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, store.Users[0].Roles[orgId], pkg.RoleAdmin)
	}, peoplePage)(t)
}

func TestDeletUser(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		waitForPeopleInitLoad(t, page)
		page.On("dialog", func(d playwright.Dialog) {
			if strings.Contains(d.Message(), "Are you sure") {
				d.Accept()
				fmt.Printf("Dialog accepted\n")
			}
		})
		john := store.Users[1]
		deleteBtn := page.Locator("#delete-user-" + john.Id + "-btn")
		cnt, err := deleteBtn.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, cnt, 1)

		_, err = page.ExpectResponse("**/organizations/users", func() error { return deleteBtn.Click() }, erTimeout)
		testutils.AssertNil(t, err)

		_, hasRole := john.Roles[orgId]
		testutils.AssertEqual(t, hasRole, false)
		cnt, err = deleteBtn.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, cnt, 0)
	}, peoplePage)(t)
}
