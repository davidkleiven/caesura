package api

import (
	"net/http"
	"net/http/httptest"
	"slices"
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
	if c.current == len(c.b) {
		c.current = 0
	}
	return int(c.b[c.current])
}

func FuzzEndpoints(f *testing.F) {
	config := pkg.NewDefaultConfig()
	config.StripeSecretKey = "" // Make sure stripe key is never set

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

	f.Fuzz(func(t *testing.T, b []byte) {
		store := pkg.GetStore(config)
		cookies := sessions.NewCookieStore([]byte("top-secret"))
		mux := Setup(store, config, cookies)
		rateLimiter := NewRateLimiter(1000.0, time.Second)
		server := httptest.NewServer(rateLimiter.Middleware(LogRequest(mux)))
		defer server.Close()

		cycler := cycligBytesBuffer{b: b}
		var cookie *http.Cookie

		for range numSubsequentCalls {
			methodNum := cycler.Next() % len(methods)
			routeNum := cycler.Next() % len(routes)

			method := methods[methodNum]
			route := routes[routeNum]

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
