package api

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/gorilla/sessions"
)

type cycligBytesBuffer struct {
	b       []byte
	current int
}

func (c *cycligBytesBuffer) Next() int {
	if len(c.b) == 0 {
		return 0
	}
	if c.current == len(c.b) {
		c.current = 0
	}
	return int(c.b[c.current])
}

func FuzzEndpoints(f *testing.F) {
	config := pkg.NewDefaultConfig()
	config.StoreType = "large-demo"
	config.StripeSecretKey = "" // Make sure stripe key is never set
	config.StripeIdProvider = "local"
	config.SmtpConfig.SendFn = pkg.NoOpSendFunc

	// List all routes except those making external requests
	routes := []string{
		RouteRoot,
		RouteUpload,
		RouteCss,
		RouteTermsConditions,
		RouteInstruments,
		RouteChoice,
		RouteJsPdfViewer,
		RouteDeleteMode,
		RouteOverview,
		RouteOverviewSearch,
		RouteOverviewProjectSelector,
		RouteProjectQueryInput,
		RouteProjects,
		RouteProjectsNames,
		RouteProjectsInfo,
		RouteProjectsId,
		RouteResources,
		RouteResourcesId,
		RouteResourcesIdContent,
		RouteResourcesIdSubmitForm,
		RouteResourcesParts,
		RouteLogin,
		RouteLoginBasic,
		RouteLoginReset,
		RouteLoginResetForm,
		RouteLogout,
		RouteOrganizations,
		RouteOrganizationsForm,
		RouteOrganizationsIdInvite,
		RouteOrganizationsOptions,
		RouteOrganizationsActiveSession,
		RouteOrganizationsUsers,
		RouteOrganizationsUsersId,
		RouteOrganizationsUsersIdGroups,
		RouteOrganizationsUsersIdRole,
		RouteOrganizationsRecipent,
		RouteSessionActiveOrganizationName,
		RouteSessionLoggedIn,
		RoutePeople,
		RoutePayment,
		RouteAbout,
		RoutePassword,
	}

	numSubsequentCalls := 40
	methods := []string{"GET", "PUT", "POST", "DELETE", "PATCH"}
	allowedCodes := []int{http.StatusOK, http.StatusBadRequest, http.StatusTooManyRequests}
	re := regexp.MustCompile(`{.*}`)

	f.Fuzz(func(t *testing.T, b []byte) {
		store := pkg.GetStore(config)
		cookies := sessions.NewCookieStore([]byte("top-secret"))
		mux := Setup(store, config, cookies)
		rateLimiter := NewRateLimiter(1000.0, time.Second)
		server := httptest.NewServer(rateLimiter.Middleware(LogRequest(mux)))
		defer server.Close()

		cycler := cycligBytesBuffer{b: b}
		var cookie *http.Cookie

		// Collect all ids that possibly can be sent to a parametrized endpoint
		storeIds := []string{}
		inMemStore, ok := store.(*pkg.MultiOrgInMemoryStore)
		if !ok {
			t.Fatal("Store is not of expected type")
		}
		for _, org := range inMemStore.Organizations {
			storeIds = append(storeIds, org.Id)
		}
		for _, singleOrgStore := range inMemStore.Data {
			for _, project := range singleOrgStore.Projects {
				storeIds = append(storeIds, project.Id())
			}
			for _, meta := range singleOrgStore.Metadata {
				storeIds = append(storeIds, meta.ResourceId())
			}
		}

		if len(storeIds) == 0 {
			t.Fatal("Store ids can not be zero")
		}

		for range numSubsequentCalls {
			methodNum := cycler.Next() % len(methods)
			routeNum := cycler.Next() % len(routes)
			idsNum := cycler.Next() % len(storeIds)
			method := methods[methodNum]
			route := routes[routeNum]

			if strings.Contains(route, "{") {
				route = re.ReplaceAllString(route, storeIds[idsNum])
			}

			req := httptest.NewRequest(method, route, nil)
			if cookie != nil {
				req.AddCookie(cookie)
			}
			rec := httptest.NewRecorder()

			server.Config.Handler.ServeHTTP(rec, req)

			if !slices.Contains(allowedCodes, rec.Code) {
				t.Errorf("Unexpected return code got: %d", rec.Code)
			}

			cookies := rec.Result().Cookies()
			if len(cookies) > 0 {
				cookie = cookies[0]
			}
		}
	})
}
