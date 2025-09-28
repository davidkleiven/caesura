package web_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/testutils"
	"github.com/playwright-community/playwright-go"
)

const organizationPage = "/organizations/form"

type OrganizationRefreshMonitor struct {
	mutex    *sync.Mutex
	targets  map[string]bool
	done     chan struct{}
	disabled bool
}

func (o *OrganizationRefreshMonitor) Observe(resp playwright.Response) {
	if o.disabled {
		return
	}
	url := resp.URL()
	o.mutex.Lock()
	defer o.mutex.Unlock()

	for target := range o.targets {
		if strings.HasSuffix(url, target) {
			fmt.Printf("Received response from target %s", url)
			o.targets[target] = true
		}
	}

	for _, seen := range o.targets {
		if !seen {
			return
		}
	}
	o.disabled = true
	close(o.done)
}

func NewOrganizationRefreshMonitor() *OrganizationRefreshMonitor {
	return &OrganizationRefreshMonitor{
		mutex: &sync.Mutex{},
		targets: map[string]bool{
			"/organizations":                    false,
			"/organizations/options":            false,
			"/session/active-organization/name": false,
		},
		done: make(chan struct{}),
	}
}

func TestOrganizationPage(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		// Wait for initial load
		currentOrganization := page.Locator("#active-organization", playwright.PageLocatorOptions{
			HasText: "My organization 2",
		})

		timeout := playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(1000),
		}

		erTimeout := playwright.PageExpectResponseOptions{
			Timeout: playwright.Float(10000),
		}

		if err := currentOrganization.WaitFor(timeout); err != nil {
			t.Fatalf("Wait for initial load: %s", err)
		}

		orgSelector := page.Locator("#existing-orgs")
		if c, err := orgSelector.Count(); c != 1 || err != nil {
			t.Fatalf("Wanted (1, nil) got (%d, %s)", c, err)
		}

		options := page.Locator("#existing-orgs option")
		if c, err := options.Count(); c != 2 || err != nil {
			t.Fatalf("Wanted (2, nil) got (%d, %s)", c, err)
		}

		t.Run("Valid invite link", func(t *testing.T) {
			invite := page.Locator("#invite-button")
			resp, err := page.ExpectResponse("**/invite", func() error {
				return invite.Click()
			}, erTimeout)
			testutils.AssertNil(t, err)

			body, err := resp.Body()
			testutils.AssertNil(t, err)
			testutils.AssertContains(t, string(body), "localhost")

			data := struct {
				InviteLink string `json:"invite_link"`
			}{}
			testutils.AssertNil(t, json.Unmarshal(body, &data))

			// Update the URL since it is set dynamically when test start
			data.InviteLink = strings.ReplaceAll(data.InviteLink, "http://localhost:8080", server.URL)
			client := http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			inviteLinkResp, err := client.Get(data.InviteLink)
			testutils.AssertNil(t, err)
			testutils.AssertEqual(t, inviteLinkResp.StatusCode, http.StatusOK)
		})

		// Set up listeners to keep track of all sequence of calls

		t.Run("Test create organization", func(t *testing.T) {
			err := page.Locator("#name").Fill("My new organization")
			testutils.AssertNil(t, err)

			createBtn := page.Locator("#create-btn")
			numOrg := len(store.Organizations)

			monitor := NewOrganizationRefreshMonitor()
			page.On("response", monitor.Observe)
			testutils.AssertNil(t, createBtn.Click())

			select {
			case <-monitor.done:
				fmt.Printf("Success")
			case <-time.After(2 * time.Second):
				t.Fatalf("Did not finish in 2 seconds")
			}
			testutils.AssertEqual(t, numOrg+1, len(store.Organizations))

			activeOrg := page.Locator("#active-organization")
			txt, err := activeOrg.TextContent()
			testutils.AssertNil(t, err)
			testutils.AssertEqual(t, txt, "My new organization")
		})

		t.Run("Test delete organization", func(t *testing.T) {
			deleteBtn := page.Locator("#delete-btn")
			page.On("dialog", func(d playwright.Dialog) {
				if strings.Contains(d.Message(), "Are you sure") {
					d.Accept()
				}
			})

			monitor := NewOrganizationRefreshMonitor()
			page.On("response", monitor.Observe)
			testutils.AssertNil(t, deleteBtn.Click())

			select {
			case <-monitor.done:
				fmt.Printf("Success")
			case <-time.After(2 * time.Second):
				t.Fatalf("Did not finish in 2 seconds")
			}

			numDeleted := 0
			for _, org := range store.Organizations {
				if org.Deleted {
					numDeleted++
				}
			}

			testutils.AssertEqual(t, numDeleted, 1)
		})
	}, organizationPage)(t)
}

func TestSubscribe(t *testing.T) {
	withBrowser(func(t *testing.T, page playwright.Page) {
		subscribeBtn := page.Locator("#subscribe-btn")
		cnt, err := subscribeBtn.Count()
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, cnt, 1)

		timeout := playwright.PageExpectResponseOptions{
			Timeout: playwright.Float(1000.0),
		}
		_, err = page.ExpectResponse("**/subscription-page", func() error { return subscribeBtn.Click() }, timeout)
		testutils.AssertNil(t, err)

		expiryField := page.Locator("#expiry-date")
		content, err := expiryField.TextContent()
		testutils.AssertNil(t, err)

		pattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$`)
		matches := pattern.MatchString(content)
		t.Logf("Expiry date %s", content)
		testutils.AssertEqual(t, matches, true)
	}, organizationPage)(t)
}
