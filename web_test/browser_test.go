package web_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/api"
	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
	"github.com/gorilla/sessions"
	"github.com/playwright-community/playwright-go"
)

var (
	server      *httptest.Server
	store       = pkg.NewDemoStore()
	cookieStore = sessions.NewCookieStore([]byte("some-random-key"))
	browser     playwright.Browser
)

const (
	uploadPage = "/upload"
	orgId      = "cccc13f9-ddd5-489e-bd77-3b935b457f71"
)

func createSignedInCookie(cookieStore *sessions.CookieStore, url string) playwright.OptionalCookie {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)

	session, err := cookieStore.Get(request, api.AuthSession)
	if err != nil {
		panic(err)
	}

	userInfo := store.Users[0]
	data, err := json.Marshal(userInfo)
	if err != nil {
		panic(err)
	}

	session.Values["role"] = data
	session.Values["orgId"] = orgId
	session.Values["userId"] = userInfo.Id
	if err := session.Save(request, recorder); err != nil {
		panic(err)
	}

	resp := recorder.Result()
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		panic("Expected only one cookie")
	}
	cookie := cookies[0]
	return playwright.OptionalCookie{
		Name:     cookie.Name,
		Value:    cookie.Value,
		HttpOnly: playwright.Bool(true),
		Secure:   playwright.Bool(false),
		URL:      playwright.String(url),
	}
}

func TestMain(m *testing.M) {
	pw, err := playwright.Run()
	if err != nil {
		fmt.Print(err)
		os.Exit(failureReturnCode())
	}
	defer pw.Stop()

	browser, err = pw.Chromium.Launch()
	if err != nil {
		fmt.Print(err)
		os.Exit(failureReturnCode())
	}
	defer browser.Close()
	fmt.Printf("Browser launched: version=%s name=%s connected=%v\n", browser.Version(), browser.BrowserType().Name(), browser.IsConnected())

	config := pkg.NewDefaultConfig()
	config.SmtpConfig.SendFn = pkg.NoOpSendFunc

	// The key must match the store used to get the cookie value
	mux := api.Setup(store, config, cookieStore)
	server = httptest.NewServer(mux)
	defer server.Close()

	fmt.Printf("Test server started. url=%s\n", server.URL)

	rcode := m.Run()
	os.Exit(rcode)
}

func withBrowser(testFunc func(t *testing.T, page playwright.Page), path string) func(t *testing.T) {
	return func(t *testing.T) {
		initialStore := store.Clone()
		defer func() {
			store.Data = initialStore.Data
			store.Organizations = initialStore.Organizations
			store.Users = initialStore.Users
		}()

		context, err := browser.NewContext()
		if err != nil {
			t.Fatal(err)
		}
		defer context.Close()

		cookie := createSignedInCookie(cookieStore, server.URL)
		err = context.AddCookies([]playwright.OptionalCookie{cookie})
		if err != nil {
			t.Fatal(err)
		}

		page, err := context.NewPage()
		if err != nil {
			t.Fatal(err)
		}

		page.On("request", func(request playwright.Request) {
			fmt.Printf("Request: %s %s\n", request.Method(), request.URL())
		})

		page.On("response", func(resp playwright.Response) {
			fmt.Printf("Response: %s, Status: %d\n", resp.URL(), resp.Status())
		})

		page.On("console", func(msg playwright.ConsoleMessage) {
			fmt.Printf("Console: %s\n", msg.Text())
		})

		if _, err := page.Goto(server.URL + path); err != nil {
			t.Fatal(err)
		}
		testFunc(t, page)
	}
}

func failureReturnCode() int {
	if _, inCi := os.LookupEnv("CI"); inCi {
		return 1
	}
	return 0
}

func TestInstrumentListIsLoaded(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		element := page.Locator("#instrument-list")
		text, err := element.TextContent()
		if err != nil {
			t.Fatal(err)

		}

		if !strings.Contains(text, "Trumpet") {
			t.Fatalf("Expepected to find 'Trumpet' in the instrument list, but got: %s", text)
		}
	}, uploadPage)(t)
}

func TestFilterList(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		err := page.Locator("input[name='token']").Fill("flu")
		if err != nil {
			t.Fatal(err)

		}

		// Trigger key-up
		if err := page.Locator("input[name='token']").Press("Enter"); err != nil {
			t.Fatal(err)

		}

		response, err := page.ExpectResponse(
			"**/instruments**",
			func() error { return nil },
			playwright.PageExpectResponseOptions{Timeout: playwright.Float(1000)},
		)

		if err != nil || !response.Ok() {
			t.Fatal(err)

		}

		element := page.Locator("#instrument-list")
		text, err := element.TextContent()
		if err != nil {
			t.Fatal(err)

		}

		for instrument := range strings.SplitSeq(text, "\n") {
			if strings.ReplaceAll(instrument, " ", "") != "" && !strings.Contains(instrument, "Flute") {
				t.Fatalf("Expected to find 'Flute' in the instrument list, but got: %s", text)
			}
		}
	}, uploadPage)(t)

}

func TestFieldPopulatedOnClick(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		trumpetElement := page.Locator("li:text('Trumpet')").First()
		if err := trumpetElement.Click(); err != nil {
			t.Fatal(err)

		}

		chosenInstrument := page.Locator("#chosen-instrument")
		text, err := chosenInstrument.TextContent()
		if err != nil {
			t.Fatal(err)

		}
		if !strings.Contains(text, "Trumpet") {
			t.Fatalf("Expected to find 'Trumpet' in the chosen instrument, but got: %s", text)
		}

	}, uploadPage)(t)
}

func loadPdf(page playwright.Page, t *testing.T) func() {
	f, err := os.CreateTemp("", "test*.pdf")
	if err != nil {
		t.Fatal(err)
		return func() {}
	}
	removeFile := func() { os.Remove(f.Name()) }
	if err := pkg.CreateNPagePdf(f, 2); err != nil {
		t.Fatal(err)
		return removeFile
	}

	if err := page.Locator("#file-input").SetInputFiles(f.Name()); err != nil {
		t.Fatal(err)
		return removeFile
	}

	waitOpts := playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(5000)}
	if _, err := page.WaitForFunction(`() => document.querySelector("#page-count").textContent === "2"`, waitOpts); err != nil {
		t.Fatalf("Failed to load PDF: %s", err)
		return removeFile
	}
	return removeFile
}

func assignPage(page playwright.Page, t *testing.T) {
	score := page.Locator("li:text('Conductor')").First()
	if err := score.Click(); err != nil {
		t.Fatal(err)
	}

	assignButton := page.Locator("#assign-page")
	assignButton.Click()

	// Now the page should be set to 2
	pageNum := page.Locator("#page-num")
	if currentPage, err := pageNum.TextContent(); err != nil || currentPage != "2" {
		t.Fatalf("Expected current page to be '2', but got: %s (err: %v)", currentPage, err)

	}
}

func populateMetaData(page playwright.Page, t *testing.T) {
	titleInput := page.Locator("#title-input")
	if err := titleInput.Fill("Brandenburg Concerto No. 3"); err != nil {
		t.Fatal(err)

	}

	composerInput := page.Locator("#composer-input")
	if err := composerInput.Fill("Johann Sebastian Bach"); err != nil {
		t.Fatal(err)

	}

	arrangerInput := page.Locator("#arranger-input")
	if err := arrangerInput.Fill("Unknown"); err != nil {
		t.Fatal(err)

	}
}

func TestLoadPdf(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		// Ensure that Page shows 0 / 0
		currentPage, err := page.Locator("#page-num").TextContent()
		if err != nil {
			t.Fatal(err)

		}

		if currentPage != "0" {
			t.Fatalf("Expected current page to be '0', but got: %s", currentPage)
		}

		pageCount, err := page.Locator("#page-count").TextContent()
		if err != nil {
			t.Fatal(err)

		}

		if pageCount != "0" {
			t.Fatalf("Expected page count to be '0', but got: %s", pageCount)
		}

		deletePdf := loadPdf(page, t)
		defer deletePdf()

		nextPage := page.Locator("#next-page")
		prevPage := page.Locator("#prev-page")

		for i, action := range []struct {
			f            func() error
			expectedPage string
		}{
			{func() error { return nil }, "1"},
			{func() error { return nextPage.Click() }, "2"},
			{func() error { return nextPage.Click() }, "2"}, // Should stay on page 2
			{func() error { return prevPage.Click() }, "1"},
			{func() error { return prevPage.Click() }, "1"}, // Should stay on page 1
		} {
			if err := action.f(); err != nil {
				t.Fatalf("Test #%d: %s", i, err)

			}

			time.Sleep(500 * time.Millisecond) // Wait for the page to update

			currentPage, err = page.Locator("#page-num").TextContent()
			if err != nil {
				t.Fatal(err)

			}
			if currentPage != action.expectedPage {
				t.Fatalf("Action #%d: Expected current page to be %s, but got: %s", i, action.expectedPage, currentPage)
			}
		}

	}, uploadPage)(t)
}

func TestAssign(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		cancel := loadPdf(page, t)
		defer cancel()

		assignButton := page.Locator("#assign-page")

		// Add response to alerts
		alertTriggered := false
		page.On("dialog", func(dialog playwright.Dialog) {
			alertTriggered = true
			dialog.Accept()
		})

		// Click the assign button when no group is selected (should trigger an alert)
		assignButton.Click()
		if !alertTriggered {
			t.Fatal("Expected alert to be triggered, but it was not.")

		}

		// Trigger population of the chosen instrument field
		trumpetElement := page.Locator("li:text('Trumpet')").First()
		if err := trumpetElement.Click(); err != nil {
			t.Fatal(err)

		}

		if txt, err := page.Locator("#chosen-instrument").TextContent(); txt != "Trumpet" || err != nil {
			t.Fatalf("Expected chosen instrument to be 'Trumpet', but got: %s (err: %v)", txt, err)

		}

		// Test that the assignment tab is updated when clicking subsequent times
		for i, expect := range []struct {
			from string
			to   string
		}{
			{from: "1", to: "1"},
			{from: "1", to: "2"},
			{from: "1", to: "2"},
		} {
			assignButton.Click()

			// Confirm that there is an element with id "trumpet"
			trumpetElementAssignment := page.Locator("#trumpet")
			if exists, err := trumpetElementAssignment.Count(); err != nil || exists != 1 {
				t.Fatalf("Click %d: Err %v number of occurences of #trumpet: %d", i, err, exists)

			}

			if txt, err := page.Locator("#trumpet-from").TextContent(); err != nil || txt != expect.from {
				t.Fatalf("Click %d: Expected #trumpet-from to be '%s', but got: %s (err: %v)", i, expect.to, txt, err)

			}

			if txt, err := page.Locator("#trumpet-to").TextContent(); err != nil || txt != expect.to {
				t.Fatalf("Click %d: Expected #trumpet-to to be '%s', but got: %s (err: %v)", i, expect.to, txt, err)

			}
		}

		// Enter delete mode
		if err := page.Locator("#delete-on-click").Check(); err != nil {
			t.Fatal("Failed to check delete mode checkbox")

		}

		// Check that assignment tab is deleted when clocking it
		if err := page.Locator("#trumpet").Click(); err != nil {
			t.Fatal(err)

		}
		if exists, err := page.Locator("#trumpet").Count(); err != nil || exists != 0 {
			t.Fatalf("Expected #trumpet to be deleted, but it still exists (count: %d, err: %v)", exists, err)

		}

		if err := page.Locator("#part-number").Fill("1 brass part"); err != nil {
			t.Fatal("Failed to fill part number field")

		}

		waitOpts := playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(5000)}

		for i, check := range []struct {
			from      string
			to        string
			alert     bool
			preAction func()
		}{
			{from: "2", to: "2", alert: false, preAction: func() {}},

			// Should trigger alert because we set the page back
			{from: "2", to: "2", alert: true, preAction: func() {
				page.Locator("#prev-page").Click()
				if _, err := page.WaitForFunction(`() => document.querySelector("#page-num").textContent === "1"`, waitOpts); err != nil {
					t.Fatalf("Failed to load PDF: %s", err)

				}
			}},
		} {
			check.preAction()
			alertTriggered = false
			assignButton.Click()
			if num, err := page.Locator("#trumpet1brasspart").Count(); err != nil || num != 1 {
				t.Fatalf("Click #%d: Expected #trumpet1brasspart to be created, but it does not exist (count: %d, err: %v)", i, num, err)

			}

			if alertTriggered != check.alert {
				t.Fatalf("Click #%d: Expected alert to be %v, but it was %v", i, check.alert, alertTriggered)

			}

			if txt, err := page.Locator("#trumpet1brasspart-from").TextContent(); err != nil || txt != check.from {
				t.Fatalf("Click #%d: Expected #trumpet1brasspart-from to be '%s', but got: %s (err: %v)", i, check.from, txt, err)

			}
			if txt, err := page.Locator("#trumpet1brasspart-to").TextContent(); err != nil || txt != check.to {
				t.Fatalf("Click #%d: Expected #trumpet1brasspart-to to be '%s', but got: %s (err: %v)", i, check.to, txt, err)

			}
		}
	}, uploadPage)(t)
}

func TestJumpToAssignedPage(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		deletePdf := loadPdf(page, t)
		defer deletePdf()

		score := page.Locator("li:text('Conductor')").First()
		if err := score.Click(); err != nil {
			t.Fatal(err)

		}
		assignPage(page, t)
		pageNum := page.Locator("#page-num")

		if err := page.Locator("#conductor").Click(); err != nil {
			t.Fatal(err)

		}

		// Now the page should be set to 1
		if currentPage, err := pageNum.TextContent(); err != nil || currentPage != "1" {
			t.Fatalf("Expected current page to be '1', but got: %s (err: %v)", currentPage, err)

		}
	}, uploadPage)(t)
}

func TestSubmit(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		deletePdf := loadPdf(page, t)
		defer deletePdf()
		assignPage(page, t)
		populateMetaData(page, t)

		if err := page.Locator("#submit-btn").Click(); err != nil {
			t.Fatal(err)

		}

		waitOpts := playwright.PageExpectResponseOptions{Timeout: playwright.Float(5000)}
		resp, err := page.ExpectResponse("**/resources**", func() error { return nil }, waitOpts)
		if err != nil {
			t.Fatalf("Failed to submit form: %s", err)

		}

		if !resp.Ok() {
			t.Fatalf("Expected response to be OK, but got status: %d", resp.Status())

		}

		flashMsg := page.Locator("#flashMessage")
		num, err := flashMsg.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, num, 1)

		text, err := flashMsg.TextContent()
		testutils.AssertNil(t, err)
		testutils.AssertContains(t, text, "File uploaded")

	}, uploadPage)(t)
}

func TestNavigateByArrowBtns(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		deletePdf := loadPdf(page, t)
		defer deletePdf()

		pageNum := page.Locator("#page-num")
		current, err := pageNum.TextContent()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, current, "1")

		err = page.Keyboard().Press("ArrowRight")
		pageSwitched := false

		// Run in a loop to give the browser some time to update
		for range 10 {
			content, err := pageNum.TextContent()
			testutils.AssertNil(t, err)
			if content == "2" {
				pageSwitched = true
				break
			}
			time.Sleep(time.Millisecond)
		}
		testutils.AssertEqual(t, pageSwitched, true)

		err = page.Keyboard().Press("ArrowLeft")
		pageSwitched = false

		// Run in a loop to give the browser some time to update
		for range 10 {
			content, err := pageNum.TextContent()
			testutils.AssertNil(t, err)
			if content == "1" {
				pageSwitched = true
				break
			}
			time.Sleep(time.Millisecond)
		}
		testutils.AssertEqual(t, pageSwitched, true)

	}, uploadPage)(t)
}

func TestAssignByPressingPlussBtn(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		deletePdf := loadPdf(page, t)
		defer deletePdf()

		// Pick part
		score := page.Locator("li:text('Conductor')").First()
		err := score.Click()
		testutils.AssertNil(t, err)

		assignments := page.Locator("#conductor-group")
		num, err := assignments.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, 0, num)

		err = page.Keyboard().Press("+")
		testutils.AssertNil(t, err)

		assignementWasAdded := false
		for range 10 {
			num, err = assignments.Count()
			testutils.AssertNil(t, err)
			if num == 1 {
				assignementWasAdded = true
				break
			}
			time.Sleep(time.Millisecond)
		}
		testutils.AssertEqual(t, assignementWasAdded, true)

	}, uploadPage)(t)

}
