package api

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/web"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

type HandlerFunc func(http.ResponseWriter, *http.Request)

const (
	AuthSession        = "auth"
	OAuthState         = "oauth_state"
	resetPasswordToken = "resetEmailToken"
	FileTimeFormat     = "20060102-150405"
)

type ctxKey string

const sessionKey ctxKey = "session"
const googleUserInfo = "https://www.googleapis.com/oauth2/v2/userinfo"
const googleToken = "https://oauth2.googleapis.com/token"

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(web.Upload(&web.ScoreMetaData{}, "en"))
}

func InstrumentSearchHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	instruments := pkg.FilterList(allInstruments(), token)
	format := r.URL.Query().Get("format")

	if format == "options" {
		slices.Sort(instruments)
		web.WriteStringAsOptions(w, instruments)
	} else {
		html := string(web.List())
		t := template.Must(template.New("list").Parse(html))

		err := t.Execute(w, IdentifiedList{Id: "instruments", Items: instruments, HxGet: "/choice", HxTarget: "#chosen-instrument", Fallback: "No items found"})
		includeError(w, http.StatusInternalServerError, "Failed to render template", err)
	}
}

func ChoiceHandler(w http.ResponseWriter, r *http.Request) {
	instrument := r.URL.Query().Get("item")

	result := instrument + "<input type=\"text\" placeholder=\"Enter part number\" id=\"part-number\"/>"
	w.Write([]byte(result))
}

func DeleteMode(w http.ResponseWriter, r *http.Request) {
	const maxSize = 1 << 12 // 4 kB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	code, err := parseForm(r)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	checkBoxValue := r.FormValue("delete-mode")
	slog.Info("Received value", "delete-mode", checkBoxValue)

	if checkBoxValue == "1" {
		w.Write([]byte("(Click to remove)"))
	} else {
		w.Write([]byte("(Click to jump)"))
	}
}

func SubmitHandler(submitter pkg.Submitter, timeout time.Duration, maxSize int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		maxUploadSize := int64(maxSize) << 20

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		err := r.ParseMultipartForm(maxUploadSize)

		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			msg := fmt.Sprintf("File is larger than max allowed size (~%d MB).", maxSize)
			http.Error(w, msg, http.StatusRequestEntityTooLarge)
			return
		} else if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			slog.Error("Failed to parse form", "error", err)
			return
		}

		file, _, err := r.FormFile("document")
		if err != nil {
			http.Error(w, "Failed to retrieve file from form", http.StatusBadRequest)
			slog.Error("Failed to retrieve file from form", "error", err)
			return
		}
		defer file.Close()

		var assignments []pkg.Assignment
		raw := r.MultipartForm.Value["assignments"]
		if len(raw) == 0 {
			http.Error(w, "No assignments provided", http.StatusBadRequest)
			slog.Error("No assignments provided")
			return
		}
		if err := json.Unmarshal([]byte(raw[0]), &assignments); err != nil {
			http.Error(w, "Failed to parse assignments", http.StatusBadRequest)
			slog.Error("Failed to parse assignments", "error", err)
			return
		}

		var metaData pkg.MetaData
		rawMeta := r.MultipartForm.Value["metadata"]

		if len(rawMeta) == 0 {
			http.Error(w, "No metadata provided", http.StatusBadRequest)
			slog.Error("No metadata provided")
			return
		}

		if err := json.Unmarshal([]byte(rawMeta[0]), &metaData); err != nil {
			msg := "Failed to parse metadata (often related to the duration input). Check that the input confirms the format 3m20s"
			http.Error(w, msg, http.StatusBadRequest)
			slog.Error("Failed to parse metadata", "error", err)
			return
		}

		resourceId := metaData.ResourceId()
		if resourceId == "" {
			http.Error(w, "Filename is empty. Note that only alphanumeric characters are allowed", http.StatusBadRequest)
			slog.Error("Filename cannot be empty.", "title", metaData.Title, "composer", metaData.Composer, "arranger", metaData.Arranger)
			return
		}

		pdfIter := pkg.SplitPdf(file, assignments)
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		orgId := MustGetOrgId(MustGetSession(r))
		if err := submitter.Submit(ctx, orgId, &metaData, pdfIter); err != nil {
			http.Error(w, "Failed to store file", http.StatusInternalServerError)
			slog.Error("Failed to store file", "error", err)
			return
		}
		slog.Info("File stored successfully", "filename", resourceId, "resourceId", resourceId, "orgId", orgId)
		w.Write([]byte("File uploaded successfully!"))
	}
}

func OverviewSearchHandler(fetcher pkg.MetaByPatternFetcher, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filterValue := r.URL.Query().Get("resource-filter")
		pattern := &pkg.MetaData{
			Title:    filterValue,
			Composer: filterValue,
			Arranger: filterValue,
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		orgId := MustGetOrgId(MustGetSession(r))
		meta, err := fetcher.MetaByPattern(ctx, orgId, pattern)
		if err != nil {
			http.Error(w, "Failed to fetch metadata", http.StatusInternalServerError)
			slog.Error("Failed to fetch metadata", "error", err)
			return
		}
		web.ResourceList(w, meta)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func OverviewHandler(w http.ResponseWriter, r *http.Request) {
	language := pkg.LanguageFromReq(r)
	w.Write(web.Overview(language))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func JsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	web.PdfJs(w)
}

func SearchProjectHandler(store pkg.ProjectByNameGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectName := r.URL.Query().Get("projectQuery")
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		orgId := MustGetOrgId(MustGetSession(r))
		project, err := store.ProjectsByName(ctx, orgId, projectName)
		slog.Info("Searching for projects", "project_name", projectName, "num_results", len(project))
		if err != nil {
			http.Error(w, "Failed to fetch project", http.StatusInternalServerError)
			slog.Error("Failed to fetch project", "error", err)
			return
		}

		html := string(web.List())
		t := template.Must(template.New("list").Parse(html))

		project_names := make([]string, len(project))
		for i, p := range project {
			project_names[i] = p.Name
		}
		lang := pkg.LanguageFromReq(r)
		pkg.PanicOnErr(t.Execute(w, IdentifiedList{Id: "projects", Items: project_names, HxGet: "/project-query-input", HxTarget: "#project-query-input", Fallback: web.CreateNewProject(lang)}))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func ProjectSelectorModalHandler(w http.ResponseWriter, r *http.Request) {
	language := pkg.LanguageFromReq(r)
	w.Write(web.ProjectSelectorModal(language))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func ProjectQueryInputHandler(w http.ResponseWriter, r *http.Request) {
	value := r.URL.Query().Get("item")
	web.ProjectQueryInput(w, "en", value)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func ProjectSubmitHandler(submitter pkg.ProjectSubmitter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			slog.Error("Failed to parse form", "error", err)
			return
		}

		projectName := r.FormValue("projectQuery")
		if projectName == "" {
			http.Error(w, "Project name cannot be empty", http.StatusBadRequest)
			slog.Error("Project name cannot be empty")
			return
		}

		resourceIds := r.Form["pieceIds"]
		project := &pkg.Project{
			Name:        projectName,
			ResourceIds: resourceIds,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		orgId := MustGetOrgId(MustGetSession(r))
		if err := submitter.SubmitProject(ctx, orgId, project); err != nil {
			http.Error(w, "Failed to submit project", http.StatusInternalServerError)
			slog.Error("Failed to submit project", "error", err)
			return
		}
		slog.Info("Project submitted successfully", "project_name", projectName, "num_resources", len(resourceIds))
		w.Write(fmt.Appendf(nil, "Added %d piece(s) to '%s'", len(resourceIds), projectName))
	}
}

func RemoveFromProject(remover pkg.ProjectResourceRemover, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectId := r.PathValue("projectId")
		resourceId := r.PathValue("resourceId")

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		orgId := MustGetOrgId(MustGetSession(r))
		if err := remover.RemoveResource(ctx, orgId, projectId, resourceId); err != nil {
			http.Error(w, "failed to remove resource", http.StatusInternalServerError)
			slog.Error("Failed to remove resource", "project-id", projectId, "resource-id", resourceId, "host", r.Host)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		msg := fmt.Sprintf("Successfully deleted item %s from project %s", resourceId, projectId)
		w.Write([]byte(msg))
	}
}

func ProjectHandler(w http.ResponseWriter, r *http.Request) {
	language := pkg.LanguageFromReq(r)
	w.Write(web.Projects(language))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func SearchProjectListHandler(store pkg.ProjectByNameGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectName := r.URL.Query().Get("projectQuery")
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		orgId := MustGetOrgId(MustGetSession(r))
		projects, err := store.ProjectsByName(ctx, orgId, projectName)
		if err != nil {
			http.Error(w, "Failed to fetch projects", http.StatusInternalServerError)
			slog.Error("Failed to fetch projects", "error", err)
			return
		}

		web.ProjectList(w, projects)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func ProjectByIdHandler(store pkg.ProjectMetaByIdGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectId := r.PathValue("id")
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		orgId := MustGetOrgId(MustGetSession(r))
		project, err := store.ProjectById(ctx, orgId, projectId)
		if err != nil {
			http.Error(w, "Failed to fetch project", http.StatusInternalServerError)
			slog.Error("Failed to fetch project", "error", err)
			return
		}

		metaData := make([]pkg.MetaData, 0, len(project.ResourceIds))
		for _, id := range project.ResourceIds {
			meta, err := store.MetaById(ctx, orgId, id)
			if err != nil {
				slog.Error("Failed to fetch metadata for project", "error", err)
			} else {
				metaData = append(metaData, *meta)
			}
		}

		web.ProjectContent(w, project, metaData, "en")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func ResourceContentByIdHandler(s pkg.ResourceGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		orgId := MustGetOrgId(MustGetSession(r))
		id := r.PathValue("id")
		downloader := pkg.NewResourceDownloader().GetMetaData(ctx, s, orgId, id).GetResource(ctx, s, orgId)

		if downloader.Error != nil {
			http.Error(w, "could not fetch resource", http.StatusInternalServerError)
			slog.Error("Failed to fetch resource", "error", downloader.Error)
		}

		content := web.ResourceContentData{
			ResourceId: id,
			Filenames:  downloader.Filenames(),
		}
		web.ResourceContent(w, &content)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
}

func ResourceDownload(s pkg.ResourceGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		session := MustGetSession(r)
		orgId := MustGetOrgId(session)
		resourceId := r.PathValue("id")
		filename := r.URL.Query().Get("file")
		downloader := pkg.NewResourceDownloader().GetMetaData(ctx, s, orgId, resourceId).GetResource(ctx, s, orgId)

		var (
			statusCode         int
			contentDisposition string
			contentType        string
		)
		if filename == "" {
			zipFilename := downloader.ZipFilename()
			contentDisposition = "attachment; filename=\"" + zipFilename + "\""
			contentType = "application/zip"
			downloader.ZipResource(w, pkg.IncludeAll)
		} else {
			contentDisposition = "attachment; filename=\"" + filename + "\""
			contentType = "application/pdf"
			downloader.ExtractSingleFile(filename, w)
		}

		err := downloader.Error
		switch {
		case errors.Is(err, pkg.ErrFileNotInZipArchive),
			errors.Is(err, pkg.ErrFileNotFound),
			errors.Is(err, pkg.ErrResourceMetadataNotFound):
			statusCode = http.StatusNotFound
		case err != nil:
			statusCode = http.StatusInternalServerError
		default:
			statusCode = http.StatusOK
		}

		if err != nil {
			http.Error(w, err.Error(), statusCode)
			slog.Error("Error during download resource", "error", err, "id", resourceId, "file", filename)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", contentDisposition)
		slog.Info("Resource downloaded")
	}
}

func AddToResourceHandler(metaGetter pkg.MetaByIdGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		orgId := MustGetOrgId(MustGetSession(r))
		meta, err := metaGetter.MetaById(ctx, orgId, id)
		if err != nil {
			http.Error(w, "Error when fetching metadata", http.StatusInternalServerError)
			slog.Error("Error when fetching metadata", "error", err, "id", id, "url", r.URL.Path)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(web.Upload(&web.ScoreMetaData{Composer: meta.Composer, Arranger: meta.Arranger, Title: meta.Title}, "en"))
	}
}

func HandleGoogleLogin(oauthConfig *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stateString := MustGenerateStateString()
		session := MustGetSession(r)
		session.Values[OAuthState] = stateString
		if err := session.Save(r, w); err != nil {
			http.Error(w, "Failed to save session "+err.Error(), http.StatusInternalServerError)
			return
		}

		url := oauthConfig.AuthCodeURL(stateString)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}

func HandleGoogleCallback(roleStore pkg.RoleStore, oauthConfig *oauth2.Config, timeout time.Duration, signSecret string, transport http.RoundTripper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := r.FormValue("state")
		session := MustGetSession(r)
		if state != session.Values[OAuthState] {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			return
		}

		code := r.FormValue("code")
		if code == "" {
			http.Error(w, "Code not found", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if transport != nil {
			ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: transport})
		}
		token, err := oauthConfig.Exchange(ctx, code)
		if err != nil {
			http.Error(w, "Code exchange failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		client := oauthConfig.Client(ctx, token)
		if transport != nil {
			client.Transport = transport
		}
		resp, err := client.Get(googleUserInfo)
		if err != nil || resp.StatusCode >= 400 {
			msg, code := CodeAndMessage(err, resp.StatusCode)
			http.Error(w, "Failed getting user info: "+msg, code)
			return
		}
		defer resp.Body.Close()

		var userInfo pkg.UserInfo
		if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
			http.Error(w, "Failed decoding user info: "+err.Error(), http.StatusInternalServerError)
			return
		}

		result := InitializeUserSession(SessionInitParams{
			Ctx:        ctx,
			Session:    session,
			User:       &userInfo,
			SignSecret: signSecret,
			Store:      roleStore,
			Writer:     w,
			Req:        r,
		})

		if result.Error != nil {
			http.Error(w, result.Error.Error(), result.ReturnCode)
			slog.Error("Could not initialize user session", "error", result.Error, "host", r.Host)
			return
		}

		redirect := "/organizations"
		slog.Info("Successfully logged in user")
		http.Redirect(w, r, redirect, http.StatusSeeOther)
	}
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	language := pkg.LanguageFromReq(r)
	w.Write(web.Index(language))
}

func OrganizationsHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	language := pkg.LanguageFromReq(r)
	w.Write(web.Organizations(language))
}

func InviteLink(baseURL, signSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgId := r.PathValue("id")

		currentTime := time.Now()
		claims := InviteClaim{
			OrgId: orgId,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(currentTime.Add(48 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(currentTime),
				NotBefore: jwt.NewNumericDate(currentTime),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signedToken, err := token.SignedString([]byte(signSecret))
		if err != nil {
			http.Error(w, "Failed to sign token", http.StatusInternalServerError)
			slog.Error("Failed to sign invite link token", "error", err, "host", r.Host)
			return
		}

		inviteURL := baseURL + "/login?invite-token=" + url.QueryEscape(signedToken)

		respBody := struct {
			InviteLink string `json:"invite_link"`
		}{
			InviteLink: inviteURL,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}
}

func OrganizationRegisterHandler(store pkg.IAMStore, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const maxSize = 4096
		r.Body = http.MaxBytesReader(w, r.Body, maxSize)
		code, err := parseForm(r)
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}

		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "Name can not be empty", http.StatusBadRequest)
			return
		}

		org := pkg.Organization{
			Id:   pkg.RandomInsecureID(),
			Name: name,
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		session := MustGetSession(r)
		userId := MustGetUserId(session)

		registrationFlow := pkg.NewRegisterOrganizationFlow(ctx, store, session)
		registrationFlow.Register(&org).RegisterAdmin(userId, org.Id).RetrieveUserInfo(userId).UpdateSession(r, w, org.Id)
		if err := registrationFlow.Error; err != nil {
			http.Error(w, "Could not register organization: "+err.Error(), http.StatusInternalServerError)
			slog.Error("Could not register organization", "error", err, "host", r.Host)
			return
		}
		slog.Info("Successfully registered new organization", "name", org.Name, "id", org.Id)
	}
}

func OptionsFromSessionHandler(orgGetter pkg.OrganizationGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := MustGetSession(r)
		roles := MustGetUserInfo(session)

		// Optionally get the organization ID
		orgId, ok := session.Values["orgId"].(string)
		if !ok {
			orgId = ""
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		options := make([]pkg.Organization, 0, len(roles.Roles))
		for id := range roles.Roles {
			org, err := orgGetter.GetOrganization(ctx, id)
			if err != nil {
				slog.Error("Could not fetch organization", "error", err, "organization-id", id)
			} else {
				options = append(options, org)
			}
		}

		slices.SortStableFunc(options, func(a pkg.Organization, b pkg.Organization) int {
			aEqual := a.Id == orgId
			bEqual := b.Id == orgId
			if aEqual {
				return -1
			} else if bEqual {
				return 1
			}
			return 0
		})

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		web.WriteOrganizationHTML(w, options)
	}
}

func DeleteOrganizationHandler(deleter pkg.OrganizationDeleter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := MustGetSession(r)
		orgId := MustGetOrgId(session)
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := deleter.DeleteOrganization(ctx, orgId); err != nil {
			http.Error(w, "Could not delete organization", http.StatusInternalServerError)
			slog.Error("Could not delete organization", "error", err, "organization-id", orgId, "host", r.Host)
			return
		}

		session.Values["orgId"] = ""
		info := MustGetUserInfo(session)
		delete(info.Roles, orgId)
		pkg.PopulateSessionWithRoles(session, info)
		if err := session.Save(r, w); err != nil {
			http.Error(w, "Could not update session", http.StatusInternalServerError)
			slog.Error("Could not update session", "error", err, "host", r.Host, "organization-id", orgId)
			return
		}
	}
}

func ChosenOrganizationSessionHandler(w http.ResponseWriter, r *http.Request) {
	orgId := r.URL.Query().Get("existing_org")
	if orgId == "" {
		http.Error(w, "No organization id passed", http.StatusBadRequest)
		slog.Error("No organization id passed", "host", r.Host)
		return
	}

	session := MustGetSession(r)
	session.Values["orgId"] = orgId
	if err := session.Save(r, w); err != nil {
		http.Error(w, "Could not save session: "+err.Error(), http.StatusInternalServerError)
		slog.Error("Coult not save session", "error", err, "host", r.Host)
		return
	}
}

func ActiveOrganization(getter pkg.OrganizationGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := MustGetSession(r)
		orgId, ok := session.Values["orgId"].(string)

		language := pkg.LanguageFromReq(r)
		noOrg := web.NoOrganization(language)
		if !ok || orgId == "" {
			w.Write([]byte(noOrg))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		org, err := getter.GetOrganization(ctx, orgId)
		if err != nil {
			slog.Error("Could not get organization", "error", err, "host", r.Host)
			w.Write([]byte(noOrg))
			return
		}
		w.Write([]byte(org.Name))
	}
}

func AllUsers(store pkg.UserGetter, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("name")
		session := MustGetSession(r)
		orgId := MustGetOrgId(session)
		userInfo := MustGetUserInfo(session)
		role := userInfo.Roles[orgId]

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		var (
			users []pkg.UserInfo
			err   error
		)
		if role == pkg.RoleAdmin {
			// Admins gets a list of all users
			users, err = store.GetUsersInOrg(ctx, orgId)
			if err != nil {
				http.Error(w, "Error when fething users "+err.Error(), http.StatusInternalServerError)
				slog.Error("Error when fething users ", "error", err, "host", r.Host)
				return
			}
		} else {
			// Other just gets information about themselves
			userInfoFromStore, err := store.GetUserInfo(ctx, userInfo.Id)
			if err != nil {
				http.Error(w, "Could not fetch user info: "+err.Error(), http.StatusInternalServerError)
				slog.Error("Could not fetch user info", "error", err, "host", r.Host, "userId", userInfo.Id)
				return
			}
			users = append(users, *userInfoFromStore)
		}

		if filter != "" {
			users = slices.DeleteFunc(users, func(u pkg.UserInfo) bool {
				email := strings.ToLower(u.Email)
				name := strings.ToLower(u.Name)
				lowerFilter := strings.ToLower(filter)
				return !strings.Contains(email, lowerFilter) && !strings.Contains(name, lowerFilter)
			})
		}
		slices.SortStableFunc(users, func(a, b pkg.UserInfo) int {
			return cmp.Compare(a.Name, b.Name)
		})

		groups := allInstruments()
		slices.Sort(groups)
		web.WriteUserList(w, users, orgId, append([]string{"-- Add to group --"}, groups...))
	}
}

func PeoplePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	web.WritePeopleHTML(w, "en")
}

func AssignRoleHandler(store pkg.RoleRegisterer, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := MustGetSession(r)
		userId := MustGetUserId(session)
		orgId := MustGetOrgId(session)
		userIdFromPath := r.PathValue("id")
		if userId == userIdFromPath {
			http.Error(w, "It is not possible to change your own role", http.StatusForbidden)
			slog.Info("User tried to change his own role", "host", r.Host)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		code, err := parseForm(r)
		if err != nil {
			http.Error(w, err.Error(), code)
			slog.Error("Failed to parse form", "error", err, "host", r.Host)
			return
		}

		role, err := strconv.Atoi(r.FormValue("role"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			slog.Error("Could not convert role into int", "error", err, "host", r.Host)
			return
		}
		if role > pkg.RoleAdmin {
			role = pkg.RoleViewer
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		err = store.RegisterRole(ctx, userIdFromPath, orgId, pkg.RoleKind(role))
		if err != nil {
			http.Error(w, "Failed to register new role: "+err.Error(), http.StatusInternalServerError)
			slog.Error("Failed to register new role", "error", err, "userId", userId, "targetUser", userIdFromPath, "orgId", orgId)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Successfully upgraded role for user"))
	}
}

func RegisterRecipent(store pkg.UserRegisterer, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessions := MustGetSession(r)
		orgId := MustGetOrgId(sessions)

		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		code, err := parseForm(r)
		if err != nil {
			http.Error(w, err.Error(), code)
			slog.Error("Failed to parse form", "error", err, "host", r.Host)
			return
		}

		user := pkg.UserInfo{
			Id:    pkg.RandomInsecureID(),
			Name:  r.FormValue("name"),
			Email: r.FormValue("email"),
			Groups: map[string][]string{
				orgId: {r.FormValue("group")},
			},
			Roles: map[string]pkg.RoleKind{orgId: pkg.RoleViewer},
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := store.RegisterUser(ctx, &user); err != nil {
			http.Error(w, "Failed to register recipent "+err.Error(), http.StatusInternalServerError)
			slog.Error("Failed to register recipent", "error", err, "host", r.Host, "orgId", orgId)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Successfully registered new recipent"))
	}
}

func DeleteUserFromOrg(store pkg.DeleteRole, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := MustGetSession(r)
		userId := MustGetUserId(session)
		orgId := MustGetOrgId(session)
		userIdFromPath := r.PathValue("id")
		if userIdFromPath == userId {
			http.Error(w, "It is not possible to delete yourself", http.StatusForbidden)
			slog.Info("User tried to delete himself", "host", r.Host)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := store.DeleteRole(ctx, userIdFromPath, orgId); err != nil {
			http.Error(w, "Could not delete role: "+err.Error(), http.StatusInternalServerError)
			slog.Error("Could not delete role", "error", err, "host", r.Host, "userId", userId, "orgId", orgId, "targetUser", userIdFromPath)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Successfully deleted user"))
	}
}

func GroupHandler(store pkg.GroupStore, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)

		code, err := parseForm(r)
		if err != nil {
			http.Error(w, err.Error(), code)
			slog.Error("Failed to parse form", "error", err)
			return
		}

		session := MustGetSession(r)
		orgId := MustGetOrgId(session)
		userInfo := MustGetUserInfo(session)
		role := userInfo.Roles[orgId]
		userIdFromPath := r.PathValue("id")
		if role < pkg.RoleAdmin && userIdFromPath != userInfo.Id {
			http.Error(w, "Only admins can edit groups of others", http.StatusUnauthorized)
			slog.Warn("Non-admin tried to edit group of another user", "orgId", orgId, "userId", userInfo.Id, "host", r.Host)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		switch r.Method {
		case http.MethodPost:
			group := r.FormValue("group")
			err = store.RegisterGroup(ctx, userIdFromPath, orgId, group)
		case http.MethodDelete:
			group := r.URL.Query().Get("group")
			err = store.RemoveGroup(ctx, userIdFromPath, orgId, group)
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			slog.Error("Failed to edit group", "error", err, "orgId", orgId, "userId", userInfo.Id, "host", r.Host, "targetUser", userIdFromPath)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Successfully edited group"))
	}
}

func LoggedIn(w http.ResponseWriter, r *http.Request) {
	s := MustGetSession(r)
	language := pkg.LanguageFromReq(r)
	_, loggedIn := s.Values["userId"].(string)
	html := web.SignIn(language)
	if loggedIn {
		html = web.SignedIn(language)
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func DownloadUserParts(store pkg.ResourceGetter, config *pkg.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 32768)
		code, err := parseForm(r)
		if err != nil {
			http.Error(w, "Failed to parse form: "+err.Error(), code)
			slog.Error("Failed to parse form", "error", err, "host", r.Host)
			return
		}
		s := MustGetSession(r)
		orgId := MustGetOrgId(s)
		ids := r.Form["resourceId"]

		ctx, cancel := context.WithTimeout(r.Context(), config.Timeout)
		defer cancel()
		fileFilter := GroupFilterFromSession(s)
		namedBuffers := make([]pkg.NamedBuffer, len(ids))

		downloader := pkg.NewResourceDownloader()

		zipFilename := fmt.Sprintf("casesura-%s.zip", time.Now().Format(FileTimeFormat))
		contentDisposition := "attachment; filename=\"" + zipFilename + "\""
		contentType := "application/zip"
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", contentDisposition)

		var numFilesInZip int
		err = pkg.ReturnOnFirstError(
			func() error {
				for i, resourceId := range ids {
					namedBuffers[i].Name = resourceId
					internalErr := downloader.
						GetMetaData(ctx, store, orgId, resourceId).
						GetResource(ctx, store, orgId).
						ZipResource(&namedBuffers[i].Buf, fileFilter).Error

					if internalErr != nil {
						return fmt.Errorf("download failed: Id=%d, resourceId=%s error=%w", i, resourceId, internalErr)
					}
				}
				return nil
			},
			func() error {
				zw := zip.NewWriter(w)
				defer zw.Close()
				var combineError error
				numFilesInZip, combineError = pkg.CombineZip(zw, namedBuffers)
				return combineError
			},
		)

		if err != nil {
			slog.Error("Failed to collect resources", "error", err)
			return
		}

		slog.Info("Resource downloaded", "numPieces", len(ids), "numFilesInZipArchive", numFilesInZip)
	}
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	language := pkg.LanguageFromReq(r)
	session := MustGetSession(r)
	inviteToken := r.URL.Query().Get(inviteTokenKey)
	if inviteToken != "" {
		session.Values[inviteTokenKey] = inviteToken
	}

	if err := session.Save(r, w); err != nil {
		http.Error(w, "Could not save session", http.StatusInternalServerError)
		slog.Error("Could not save session", "error", err, "host", r.Host)
		return
	}
	web.LoginForm(w, language)
}

func LoginByPassword(store pkg.BasicAuthRoleStore, signSecret string, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		defer r.Body.Close()
		code, err := parseForm(r)
		if err != nil {
			http.Error(w, err.Error(), code)
			slog.Error("Error parsing form", "error", err, "host", r.Host)
			return
		}

		language := pkg.LanguageFromReq(r)
		email := r.FormValue("email")
		password := r.FormValue("password")
		retypedPassword := r.FormValue("retyped")
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		var (
			ok   bool
			user pkg.UserInfo
		)
		basicAuthCommonParams := BasicAuthCommonParams{
			Ctx:      ctx,
			Email:    email,
			Password: password,
			Writer:   w,
			Language: language,
		}
		if retypedPassword != "" {
			params := BasicAuthUserNewUser{
				BasicAuthCommonParams: basicAuthCommonParams,
				RetypedPassword:       retypedPassword,
				Store:                 store,
			}
			user, ok = RegisterNewUserByPassword(params)
		} else {
			basicAuthParams := BasicAuthUserLoginParams{
				BasicAuthCommonParams: basicAuthCommonParams,
				Store:                 store,
			}
			user, ok = LoginUserByPassword(basicAuthParams)
		}

		if !ok {
			// Return login functions are responsible of populating the
			// ResponseWriter
			return
		}

		session := MustGetSession(r)

		params := SessionInitParams{
			Ctx:        ctx,
			Session:    session,
			User:       &user,
			SignSecret: signSecret,
			Store:      store,
			Writer:     w,
			Req:        r,
		}
		result := InitializeUserSession(params)
		if result.Error != nil {
			http.Error(w, result.Error.Error(), result.ReturnCode)
			slog.Error("Error while initializing user by password", "error", result.Error, "host", r.Host)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(web.SuccessfulLogin(language)))
	}
}

func ResetPasswordEmail(config *pkg.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		defer r.Body.Close()
		code, err := parseForm(r)
		if err != nil {
			fmt.Fprintf(w, "Status: %d. Message: %s", code, err)
			slog.Error("Error parsing form", "error", err, "host", r.Host)
			return
		}

		language := pkg.LanguageFromReq(r)
		emailAddr := r.FormValue("email")
		if !validEmail(emailAddr) {
			web.EnterValidEmail(w, language)
			return
		}

		email := pkg.Email{
			Sender:    config.EmailSender,
			SmtpHost:  config.SmtpConfig.Host,
			SmtpPort:  config.SmtpConfig.Port,
			SmtpAuth:  config.SmtpConfig.Auth,
			Recipents: []string{emailAddr},
			SendFn:    config.SmtpConfig.SendFn,
		}

		ctx, cancel := context.WithTimeout(r.Context(), config.Timeout)
		defer cancel()

		var (
			signedToken  string
			emailContent *bytes.Buffer
		)
		overallErr := pkg.ReturnOnFirstError(
			func() error {
				var err error
				signedToken, err = SignedResetToken(emailAddr, config.CookieSecretSignKey, 20*time.Minute)
				return err
			},
			func() error {
				var err error
				url := config.BaseURL + "/login/reset/form?token=" + signedToken
				emailContent, err = email.Build("Caesura: reset password", "Reset link: "+url, func(yield func(string, io.Reader) bool) {})
				return err
			},
			func() error {
				return email.Send(ctx, emailContent.Bytes())
			},
		)

		if overallErr != nil {
			fmt.Fprintf(w, "Error: %s", overallErr)
			return
		}
		web.ResetEmailSent(w, language, emailAddr)
	}
}

func ResetPasswordForm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	session := MustGetSession(r)
	session.Values[resetPasswordToken] = token
	if err := session.Save(r, w); err != nil {
		slog.Error("Could not save session", "error", err)
		fmt.Fprintf(w, "Internal server error: %s", err)
		return
	}
	lang := pkg.LanguageFromReq(r)
	web.ResetPasswordPage(w, lang)
}

func UpdatePassword(store pkg.BasicAuthPasswordResetter, signSecret string, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := MustGetSession(r)
		jwtToken, ok := session.Values[resetPasswordToken].(string)
		if !ok {
			fmt.Fprintf(w, "Invalid reset token: could not convert into string")
			slog.Error("Invalid reset token: could not convert into string")
			return
		}

		email, err := emailFromResetPasswordJwt(jwtToken, signSecret)
		if err != nil {
			fmt.Fprintf(w, "Invalid JWT token: %s", err)
			slog.Warn("Invalid JWT token", "error", err)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		lang := pkg.LanguageFromReq(r)

		r.Body = http.MaxBytesReader(w, r.Body, 512)
		code, err := parseForm(r)
		if err != nil {
			fmt.Fprintf(w, "Http error (%d): %s", code, err)
			return
		}
		password := r.FormValue("password")
		retyped := r.FormValue("retyped")

		params := BasicAuthResetPasswordParams{
			BasicAuthCommonParams: BasicAuthCommonParams{
				Ctx:      ctx,
				Email:    email,
				Language: lang,
				Password: password,
				Writer:   w,
			},
			RetypedPassword: retyped,
			Store:           store,
		}
		if err := ResetUserPassword(params); err != nil {
			return
		}
		delete(session.Values, resetPasswordToken)

		if err := session.Save(r, w); err != nil {
			fmt.Fprintf(w, "Internal server error: %s", err)
			return
		}
		fmt.Fprintf(w, "Password successfully reset. Return to the login page")
	}
}

func SignOut(w http.ResponseWriter, r *http.Request) {
	session := MustGetSession(r)

	// Make a copy to avoid editing options for other sessions
	opts := *session.Options
	opts.MaxAge = -1
	session.Options = &opts
	if err := session.Save(r, w); err != nil {
		fmt.Fprintf(w, "Internal server error: %s", err)
		return
	}
	w.Write([]byte("Logged out, session cleared"))
}

func AboutUs(w http.ResponseWriter, r *http.Request) {
	lang := pkg.LanguageFromReq(r)
	web.AboutUsPage(w, lang)
}

func Setup(store pkg.Store, config *pkg.Config, cookieStore *sessions.CookieStore) *http.ServeMux {
	sessionOpt := config.SessionOpts()
	readRoute := RequireRead(cookieStore, sessionOpt)
	writeRoute := RequireWrite(config, cookieStore, sessionOpt)
	adminRoute := RequireAdmin(config, cookieStore, sessionOpt)
	adminWithoutSubscription := RequireAdminWithoutSubscription(cookieStore, sessionOpt)

	signedInRoute := RequireSignedIn(cookieStore, sessionOpt) // Require user to be signed in, but not to have a role
	userInfoRoute := RequireUserInfo(cookieStore, sessionOpt) // Require the info about user, but nessecarily a active orgId

	mux := http.NewServeMux()
	mux.HandleFunc("/", RootHandler)
	mux.HandleFunc("/upload", UploadHandler)
	mux.Handle("/css/", web.CssServer())
	mux.HandleFunc("/instruments", InstrumentSearchHandler)
	mux.HandleFunc("/choice", ChoiceHandler)
	mux.HandleFunc("/js/pdf-viewer.js", JsHandler)
	mux.HandleFunc("/delete-mode", DeleteMode)

	mux.HandleFunc("/overview", OverviewHandler)
	mux.Handle("/overview/search", readRoute(OverviewSearchHandler(store, config.Timeout)))
	mux.HandleFunc("/overview/project-selector", ProjectSelectorModalHandler)

	mux.HandleFunc("/project-query-input", ProjectQueryInputHandler)
	mux.Handle("/js/", web.JsServer())

	mux.HandleFunc("GET /projects", ProjectHandler)
	mux.Handle("GET /projects/names", readRoute(SearchProjectHandler(store, config.Timeout)))
	mux.Handle("GET /projects/info", readRoute(SearchProjectListHandler(store, config.Timeout)))
	mux.Handle("GET /projects/{id}", readRoute(ProjectByIdHandler(store, config.Timeout)))
	mux.Handle("POST /projects", writeRoute(ProjectSubmitHandler(store, config.Timeout)))
	mux.Handle("DELETE /projects/{projectId}/{resourceId}", writeRoute(RemoveFromProject(store, config.Timeout)))

	mux.Handle("GET /resources/{id}", readRoute(ResourceDownload(store, config.Timeout)))
	mux.Handle("GET /resources/{id}/content", readRoute(ResourceContentByIdHandler(store, config.Timeout)))
	mux.Handle("GET /resources/{id}/submit-form", readRoute(AddToResourceHandler(store, config.Timeout)))
	mux.Handle("POST /resources", writeRoute(SubmitHandler(store, config.Timeout, int(config.MaxRequestSizeMb))))
	mux.Handle("POST /resources/parts", writeRoute(DownloadUserParts(store, config)))

	oauthCfg := config.OAuthConfig()
	requireAuthSession := RequireSession(cookieStore, AuthSession, sessionOpt)
	mux.Handle("/login", requireAuthSession(http.HandlerFunc(LoginHandler)))
	mux.Handle("/login/google", requireAuthSession(HandleGoogleLogin(oauthCfg)))
	mux.Handle("/login/basic", requireAuthSession(LoginByPassword(store, config.CookieSecretSignKey, config.Timeout)))
	mux.Handle("POST /login/reset", ResetPasswordEmail(config))
	mux.Handle("POST /logout", requireAuthSession(http.HandlerFunc(SignOut)))
	mux.Handle("GET /login/reset/form", requireAuthSession(http.HandlerFunc(ResetPasswordForm)))
	mux.Handle("PUT /password", requireAuthSession(UpdatePassword(store, config.CookieSecretSignKey, config.Timeout)))
	mux.Handle("/auth/callback", requireAuthSession(HandleGoogleCallback(store, oauthCfg, config.Timeout, config.CookieSecretSignKey, config.Transport)))

	mux.HandleFunc("GET /organizations/form", OrganizationsHandler)
	mux.Handle("POST /organizations", signedInRoute(OrganizationRegisterHandler(store, config.Timeout)))
	mux.Handle("DELETE /organizations", adminRoute(DeleteOrganizationHandler(store, config.Timeout)))
	mux.Handle("GET /organizations/{id}/invite", adminRoute(InviteLink(config.BaseURL, config.CookieSecretSignKey)))
	mux.Handle("GET /organizations/options", userInfoRoute(OptionsFromSessionHandler(store, config.Timeout)))
	mux.Handle("GET /organizations/active/session", userInfoRoute(http.HandlerFunc(ChosenOrganizationSessionHandler)))
	mux.Handle("GET /organizations/users", readRoute(AllUsers(store, config.Timeout)))
	mux.Handle("DELETE /organizations/users/{id}", adminRoute(DeleteUserFromOrg(store, config.Timeout)))
	mux.Handle("POST /organizations/recipent", adminRoute(RegisterRecipent(store, config.Timeout)))
	mux.Handle("POST /organizations/users/{id}/groups", readRoute(GroupHandler(store, config.Timeout)))
	mux.Handle("DELETE /organizations/users/{id}/groups", readRoute(GroupHandler(store, config.Timeout)))
	mux.Handle("POST /organizations/users/{id}/role", adminRoute(AssignRoleHandler(store, config.Timeout)))

	mux.Handle("GET /session/active-organization/name", requireAuthSession(ActiveOrganization(store, config.Timeout)))
	mux.Handle("GET /session/logged-in", requireAuthSession(http.HandlerFunc(LoggedIn)))

	mux.HandleFunc("GET /people", PeoplePage)
	mux.Handle("POST /subscription-page", adminWithoutSubscription(checkoutSessionHandler(config)))
	mux.Handle("GET /subscription", readRoute(Subscription(store, config.Timeout)))
	mux.Handle("POST /payment", stripeWebhookHandler(store, config))

	mux.Handle("GET /about", http.HandlerFunc(AboutUs))
	return mux
}
