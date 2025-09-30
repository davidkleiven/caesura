package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/testutils"
	"github.com/davidkleiven/caesura/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestUploadHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)
	UploadHandler(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)
	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
	}

	if !strings.Contains(recorder.Body.String(), "Caesura") {
		t.Fatal("Expected response body to contain 'Caesura'")
	}

}

func TestInstrumentSearchHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/search?token=flute", nil)
	InstrumentSearchHandler(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if recorder.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'text/plain; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

	}

	if !strings.Contains(recorder.Body.String(), "Flute") {
		t.Fatal("Expected response body to contain 'Flute'")

	}

	if strings.Contains(recorder.Body.String(), "Trumpet") {
		t.Fatal("Expected response body to not contain 'Trumpet'")

	}
}

func TestInstrumentHandlerFormatOptions(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/instruments?format=options", nil)
	InstrumentSearchHandler(recorder, request)
	testutils.AssertEqual(t, recorder.Code, http.StatusOK)
	testutils.AssertContains(t, recorder.Body.String(), "<option", "Flute", "</option>")
}

func TestChoiceHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/choice?item=flute", nil)
	ChoiceHandler(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	expectedResponse := "flute<input type=\"text\" placeholder=\"Enter part number\" id=\"part-number\"/>"
	if recorder.Body.String() != expectedResponse {
		t.Fatalf("Expected response body to be '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}

func TestDeleteMode(t *testing.T) {
	for _, test := range []struct {
		value    string
		expected string
	}{
		{"1", "(Click to remove)"},
		{"0", "(Click to jump)"},
		{"", "(Click to jump)"},
	} {
		form := url.Values{}
		form.Set("delete-mode", test.value)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/delete-mode", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		DeleteMode(recorder, request)

		if recorder.Code != 200 {
			t.Fatalf("Expected status code 200, got %d", recorder.Code)

		}

		if recorder.Body.String() != test.expected {
			t.Fatalf("Expected response body to be '%s', got '%s'", test.expected, recorder.Body.String())

		}
	}
}

func TestDeleteModeWhenFormNotPopulated(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/delete-mode", strings.NewReader("bad=%ZZ")) // malformed encoding
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	DeleteMode(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400, got %d", recorder.Code)

	}

	expectedError := "invalid URL"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestDeleteModeTooLargeForm(t *testing.T) {
	form := url.Values{}

	for i := range 500 {
		form.Set(fmt.Sprintf("delete-mode%d", i), "1")
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/delete-mode", strings.NewReader(form.Encode())) // malformed encoding
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	DeleteMode(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Expected return code '%d' got '%d'", http.StatusRequestEntityTooLarge, recorder.Code)
	}
}

func TestSubmitBadRequestHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/resources", nil)
	request.Header.Set("Content-Type", "multipart/form-data")

	handler := SubmitHandler(pkg.NewMultiOrgInMemoryStore(), 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400, got %d", recorder.Code)

	}

	expectedError := "Failed to parse form"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func withPdf(w *multipart.Writer) {
	w.CreateFormField("filename.pdf")
	contentWriter, err := w.CreateFormFile("document", "filename.pdf")
	if err != nil {
		panic(err)
	}
	pkg.CreateNPagePdf(contentWriter, 10)
}

func withInvalidPdf(w *multipart.Writer) {
	w.CreateFormField("filename.txt")
	contentWriter, err := w.CreateFormFile("document", "filename.txt")
	if err != nil {
		panic(err)
	}
	contentWriter.Write([]byte("This is not a PDF file."))
	// Note: This is intentionally not a valid PDF to test error handling
}

func withInvalidMetaData(w *multipart.Writer) {
	metaDataWriter, err := w.CreateFormField("metadata")
	if err != nil {
		panic(err)
	}
	// Invalid JSON for metadata
	metaDataWriter.Write([]byte("invalid json"))
}

func withAssignments(w *multipart.Writer) {
	assignments := []pkg.Assignment{
		{Id: "Part1", From: 1, To: 5},
		{Id: "Part2", From: 6, To: 10},
	}
	assignmentWriter, err := w.CreateFormField("assignments")
	if err != nil {
		panic(err)
	}
	jsonBytes, err := json.Marshal(assignments)
	if err != nil {
		panic(err)
	}
	assignmentWriter.Write(jsonBytes)
}

func withMetaData(w *multipart.Writer) {
	metaDataWriter, err := w.CreateFormField("metadata")
	if err != nil {
		panic(err)
	}
	metaData := pkg.MetaData{
		Title:    "Brandenburg Concerto No. 3",
		Composer: "Johan Sebastian Bach",
		Arranger: "",
	}
	metaDataBytes, err := json.Marshal(metaData)
	if err != nil {
		panic(err)
	}
	metaDataWriter.Write(metaDataBytes)
}

func withEmptyMetaData(w *multipart.Writer) {
	metaDataWriter, err := w.CreateFormField("metadata")
	if err != nil {
		panic(err)
	}
	// Empty metadata
	metaDataWriter.Write([]byte("{}"))
}

func multipartForm(opts ...func(w *multipart.Writer)) (*bytes.Buffer, string) {
	var multipartBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBuffer)

	for _, opt := range opts {
		opt(multipartWriter)
	}

	if err := multipartWriter.Close(); err != nil {
		panic(err)
	}
	return &multipartBuffer, multipartWriter.FormDataContentType()
}

func validMultipartForm() (*bytes.Buffer, string) {
	return multipartForm(
		withPdf,
		withAssignments,
		withMetaData,
	)
}

func withAuthSession(r *http.Request, orgId string) *http.Request {
	store := sessions.NewCookieStore([]byte("whatever-key"))
	session, err := store.Get(r, AuthSession)
	if err != nil {
		panic(err)
	}

	userInfo := pkg.UserInfo{
		Id: "0000-0000",
		Roles: map[string]pkg.RoleKind{
			orgId: pkg.RoleAdmin,
		},
	}

	data, err := json.Marshal(userInfo)
	if err != nil {
		panic(err)
	}

	session.Values["role"] = data
	session.Values["orgId"] = orgId

	ctx := context.WithValue(r.Context(), sessionKey, session)
	return r.WithContext(ctx)
}

func TestSubmitHandlerValidRequest(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	inMemStore.RegisterOrganization(context.Background(), &pkg.Organization{Id: "orgId"})
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := validMultipartForm()
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)
	request = withAuthSession(request, "orgId")

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	expectedResponse := "File uploaded successfully"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}

	if len(inMemStore.Data) != 1 {
		t.Fatalf("Expected 1 file in store, got %d", len(inMemStore.Data))

	}

	// Check content in the store
	content := inMemStore.Data["orgId"]
	testutils.AssertEqual(t, len(content.Data), 2)
}

func TestSubmitHandlerInvalidJson(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	var multipartBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBuffer)
	multipartWriter.CreateFormField("filename.pdf")
	contentWriter, err := multipartWriter.CreateFormFile("document", "filename.pdf")
	if err != nil {
		t.Fatal(err)

	}
	pkg.CreateNPagePdf(contentWriter, 10)

	// Invalid JSON for assignments
	assignments := "invalid json"
	assignmentWriter, err := multipartWriter.CreateFormField("assignments")
	if err != nil {
		t.Fatal(err)

	}
	assignmentWriter.Write([]byte(assignments))
	if err := multipartWriter.Close(); err != nil {
		t.Fatal(err)

	}

	request := httptest.NewRequest("POST", "/resources", &multipartBuffer)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400, got %d", recorder.Code)

	}

	expectedError := "Failed to parse assignments"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestSubmitFormWithoutDocument(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	var multipartBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBuffer)
	if err := multipartWriter.Close(); err != nil {
		t.Fatal(err)

	}

	request := httptest.NewRequest("POST", "/resources", &multipartBuffer)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400, got %d", recorder.Code)

	}

	expectedError := "Failed to retrieve file from form"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestSubmitNonPdfFileAsDocument(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withInvalidPdf, withAssignments, withMetaData)

	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)
	request = withAuthSession(request, "orgId")

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500, got %d", recorder.Code)

	}

	expectedError := "Failed to store"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestSubmitHandlerNoAssignments(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withPdf, withMetaData)
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400, got %d", recorder.Code)

	}

	expectedError := "No assignments provided"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestSubmitHandlerNoMetaData(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withPdf, withAssignments)
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400, got %d", recorder.Code)

	}

	expectedError := "No metadata provided"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestSubmitHandlerInvalidMetaData(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withPdf, withAssignments, withInvalidMetaData)
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400, got %d", recorder.Code)

	}

	expectedError := "Failed to parse metadata"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

type failingSubmitter struct {
	err error
}

func (f *failingSubmitter) Submit(ctx context.Context, orgId string, meta *pkg.MetaData, i iter.Seq2[string, []byte]) error {
	return f.err
}

func TestSubmitHandlerStoreErrors(t *testing.T) {
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := validMultipartForm()
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)
	request = withAuthSession(request, "someOrg")

	handler := SubmitHandler(&failingSubmitter{err: errors.New("what??")}, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500, got %d", recorder.Code)

	}
}

func TestEntityTooLargeWhenUploadIsTooLarge(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm()
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	handler := SubmitHandler(inMemStore, 10*time.Second, 0)
	handler(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Expected code %d got %d", http.StatusRequestEntityTooLarge, recorder.Code)
	}
}

func TestBadRequestOnEmptyResourceId(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withInvalidPdf, withAssignments, withEmptyMetaData)
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	handler := SubmitHandler(inMemStore, 10*time.Second, 4096)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected code %d got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestOverHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/overview", nil)

	OverviewHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

	}

	if !strings.Contains(recorder.Body.String(), "Title") {
		t.Fatal("Expected response body to contain 'Title'")

	}
}

func TestOverviewSearchHandler(t *testing.T) {

	for _, test := range []struct {
		resourceFilter string
		expectedCount  int
	}{
		{"", 2},             // No filter, expect all resources
		{"arranger+x", 1},   // Filter by arranger
		{"demo+title+1", 1}, // Filter by title
		{"nonexistent", 0},  // Non-existent filter, expect no results
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", fmt.Sprintf("/overview/search?resource-filter=%s", test.resourceFilter), nil)
		store := pkg.NewDemoStore()
		request = withAuthSession(request, store.FirstOrganizationId())

		handler := OverviewSearchHandler(store, 10*time.Second)
		handler(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("Expected status code 200, got %d", recorder.Code)

		}

		if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
			t.Fatalf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

		}

		numRows := strings.Count(recorder.Body.String(), "<tr id=\"row")
		if numRows != test.expectedCount {
			t.Fatalf("Expected %d rows in response, got %d", test.expectedCount, numRows)

		}
	}
}

type failingFetcher struct {
	err error
}

func (f *failingFetcher) MetaByPattern(ctx context.Context, orgId string, pattern *pkg.MetaData) ([]pkg.MetaData, error) {
	return nil, f.err
}

func TestInternalServerErrorOnFailure(t *testing.T) {
	expectedError := errors.New("fetch error")
	recorder := httptest.NewRecorder()

	request := httptest.NewRequest("GET", "/overview/search?resource-filter=flute", nil)
	request = withAuthSession(request, "someOrg")
	handler := OverviewSearchHandler(&failingFetcher{err: expectedError}, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500, got %d", recorder.Code)

	}
}

func TestSearchProjectHandler(t *testing.T) {
	store := pkg.NewInMemoryStore()
	store.Projects["test_project"] = pkg.Project{
		Name:        "Test Project",
		ResourceIds: []string{"resource1", "resource2"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	inMemStore := pkg.NewMultiOrgInMemoryStore()
	inMemStore.Data["org1"] = store

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects/names?projectQuery=test", nil)
	request = withAuthSession(request, "org1")

	handler := SearchProjectHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

	}

	if !strings.Contains(recorder.Body.String(), "Test Project") {
		t.Fatal("Expected response body to contain 'Test Project'")

	}
}

type failingProjectByNamer struct {
	err error
}

func (f *failingProjectByNamer) ProjectsByName(ctx context.Context, orgId string, name string) ([]pkg.Project, error) {
	return nil, f.err
}

func TestSearchProjectHandlerInternelServerErrorOnFailure(t *testing.T) {
	expectedError := errors.New("fetch error")
	recorder := httptest.NewRecorder()

	request := httptest.NewRequest("GET", "/projects/names?projectQuery=test", nil)
	request = withAuthSession(request, "someOrg")
	handler := SearchProjectHandler(&failingProjectByNamer{err: expectedError}, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500, got %d", recorder.Code)

	}

	expectedResponse := "Failed to fetch project"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}

func TestProjectSelectorModalHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/overview/project-selector", nil)

	ProjectSelectorModalHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

	}

	if !strings.Contains(recorder.Body.String(), "Confirm") {
		t.Fatal("Expected response body to contain 'Confirm'")

	}
}

func TestProjectSubmitHandler(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	orgId := "someId"
	inMemStore.RegisterOrganization(context.Background(), &pkg.Organization{Id: orgId})
	recorder := httptest.NewRecorder()

	form := url.Values{}
	form.Set("projectQuery", "Test Project")
	request := httptest.NewRequest("POST", "/projects", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = withAuthSession(request, orgId)

	handler := ProjectSubmitHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if len(inMemStore.Data[orgId].Projects) != 1 {
		t.Fatalf("Expected 1 project in store, got %d", len(inMemStore.Data[orgId].Projects))

	}

	if inMemStore.Data[orgId].Projects["testproject"].Name != "Test Project" {
		t.Fatalf("Expected project name 'Test Project', got '%s'", inMemStore.Data[orgId].Projects["test_project"].Name)
	}
}

func TestBadRequestOnMissingName(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	form := url.Values{}
	request := httptest.NewRequest("POST", "/projects", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	handler := ProjectSubmitHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400, got %d", recorder.Code)

	}
}

type failingProjectSubmitter struct {
	err error
}

func (f *failingProjectSubmitter) SubmitProject(ctx context.Context, orgId string, project *pkg.Project) error {
	return f.err
}

func TestInternaltServerErrorOnProjectSubmitFailure(t *testing.T) {
	expectedError := errors.New("submit error")
	recorder := httptest.NewRecorder()

	inMemStore := &failingProjectSubmitter{err: expectedError}
	form := url.Values{}
	form.Set("projectQuery", "Test Project")
	request := httptest.NewRequest("POST", "/projects", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = withAuthSession(request, "someOrg")

	handler := ProjectSubmitHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500, got %d", recorder.Code)

	}

	expectedResponse := "Failed to submit project"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}

func TestBadRequestWhenWrongApplicationType(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects?bad=%ZZ", nil)

	inMemStore := pkg.NewMultiOrgInMemoryStore()
	handler := ProjectSubmitHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected status code 400, got %d", recorder.Code)

	}

	expectedError := "Failed to parse form"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestProjectQueryInputHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/project-query-input?item=Test%20Project", nil)

	ProjectQueryInputHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

	}

	if !strings.Contains(recorder.Body.String(), "Test Project") {
		t.Fatal("Expected response body to contain 'Test Project'")

	}
}

func TestJsHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/js/pdf-viewer.js", nil)

	JsHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if recorder.Header().Get("Content-Type") != "application/javascript; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'application/javascript; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

	}
}

func TestProjectHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects", nil)

	ProjectHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

	}
}

func TestSearchProjectListHandler(t *testing.T) {
	inMemStore := pkg.NewInMemoryStore()
	inMemStore.Projects["test_project"] = pkg.Project{
		Name:        "Test Project",
		ResourceIds: []string{"resource1", "resource2"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	multiStore := pkg.NewMultiOrgInMemoryStore()
	multiStore.Data["org1"] = inMemStore

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects/info?projectQuery=test", nil)
	request = withAuthSession(request, "org1")

	handler := SearchProjectListHandler(multiStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

	}

	if !strings.Contains(recorder.Body.String(), "Test Project") {
		t.Fatal("Expected response body to contain 'Test Project'")

	}
}

func TestSearchProjectListInternalServerError(t *testing.T) {
	expectedError := errors.New("fetch error")
	recorder := httptest.NewRecorder()

	inMemStore := &failingProjectByNamer{err: expectedError}
	request := httptest.NewRequest("GET", "/projects/info?projectQuery=test", nil)
	request = withAuthSession(request, "someOrg")
	handler := SearchProjectListHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500, got %d", recorder.Code)

	}

	expectedResponse := "Failed to fetch projects"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}

func TestProjectByIdHandler(t *testing.T) {
	inMemStore := pkg.NewDemoStore()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects/demoproject1", nil)
	request = withAuthSession(request, inMemStore.FirstOrganizationId())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /projects/{id}", ProjectByIdHandler(inMemStore, 10*time.Second))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))

	}

	if !strings.Contains(recorder.Body.String(), "Demo Project 1") {
		t.Fatal("Expected response body to contain 'Demo Project 1'")

	}
}

type failingProjectByIdFetcher struct {
	projectErr error
	metaErr    error
}

func (f *failingProjectByIdFetcher) ProjectById(ctx context.Context, orgId, id string) (*pkg.Project, error) {
	return &pkg.Project{Name: "Concert No. 1", ResourceIds: []string{"id1"}}, f.projectErr
}
func (f *failingProjectByIdFetcher) MetaById(ctx context.Context, orgId, id string) (*pkg.MetaData, error) {
	return nil, f.metaErr
}

func TestProjectByIdInternalServerError(t *testing.T) {
	expectedError := errors.New("fetch error")
	recorder := httptest.NewRecorder()

	inMemStore := &failingProjectByIdFetcher{projectErr: expectedError}
	request := httptest.NewRequest("GET", "/projects/test_project", nil)
	request = withAuthSession(request, "someOrg")
	handler := ProjectByIdHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status code 500, got %d", recorder.Code)

	}

	expectedResponse := "Failed to fetch project"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Fatalf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}

func TestProjectByIdMetaDataError(t *testing.T) {
	expectedError := errors.New("meta fetch error")
	recorder := httptest.NewRecorder()

	inMemStore := &failingProjectByIdFetcher{metaErr: expectedError}
	request := httptest.NewRequest("GET", "/projects/test_project", nil)
	request = withAuthSession(request, "someOrg")
	handler := ProjectByIdHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}

	if !strings.Contains(recorder.Body.String(), "Concert No. 1") {
		t.Fatal("Expected response body to contain 'Concert No. 1'")

	}
}

func TestSetup(t *testing.T) {
	store := sessions.NewCookieStore([]byte("some-random-key"))
	config := pkg.NewDefaultConfig()
	mux := Setup(pkg.NewDemoStore(), config, store)

	req, _ := http.NewRequest("GET", "/upload", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", recorder.Code)

	}
}

func TestResourceContentByIdHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	store := pkg.NewDemoStore()

	orgId := store.FirstOrganizationId()

	id := store.Data[orgId].Metadata[0].ResourceId()

	request := httptest.NewRequest("GET", "/resources/"+id+"/content", nil)
	request = withAuthSession(request, orgId)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /resources/{id}/content", ResourceContentByIdHandler(store, 1*time.Second))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected return code '200' got %d", recorder.Code)
	}

	tokens := []string{"Part0", "Part2", "Part3", "Part4"}
	body := recorder.Body.String()
	for i, token := range tokens {
		if !strings.Contains(body, token) {
			t.Fatalf("Test #%d: expected %s to be part of\n%s\n", i, token, body)
		}
	}
}

func TestResourceDownloaderFullZipDownload(t *testing.T) {
	store := pkg.NewDemoStore()

	orgId := store.FirstOrganizationId()

	resourceId := store.Data[orgId].Metadata[0].ResourceId()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/resources/"+resourceId, nil)
	request = withAuthSession(request, orgId)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /resources/{id}", ResourceDownload(store, 1*time.Second))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected code %d got %d", http.StatusOK, recorder.Code)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/zip" {
		t.Fatalf("Expected content type'application/zip'  got %s", contentType)
	}

	resourceName := store.Data[orgId].Metadata[0].ResourceId()
	if disp := recorder.Header().Get("Content-Disposition"); !strings.Contains(disp, resourceName) {
		t.Fatalf("Expected Content-Disposition to contain %s got %s", resourceName, disp)
	}

	bodyBytes, err := io.ReadAll(recorder.Body)
	if err != nil {
		t.Fatal(err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
	if err != nil {
		t.Fatal(err)
	}

	if len(zipReader.File) != 5 {
		t.Fatalf("Expected 5 files got %d", len(zipReader.File))
	}
}

func TestResourceDownloadSingleFile(t *testing.T) {
	store := pkg.NewDemoStore()

	orgId := store.FirstOrganizationId()

	resourceId := store.Data[orgId].Metadata[0].ResourceId()
	file := "Part2.pdf"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", fmt.Sprintf("/resources/%s?file=%s", resourceId, file), nil)
	request = withAuthSession(request, orgId)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /resources/{id}", ResourceDownload(store, 1*time.Second))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected code '%d' got %d", http.StatusOK, recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if contentType != "application/pdf" {
		t.Fatalf("Expected content type 'application/pdf' got %s", contentType)
	}

	contentDisp := recorder.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisp, file) {
		t.Fatalf("Expected Content-Disposition to containt '%s' got %s", file, contentDisp)
	}

	bodyBytes, err := io.ReadAll(recorder.Body)
	if err != nil {
		t.Fatal(err)
	}

	reader := bytes.NewReader(bodyBytes)
	ctx, err := api.ReadValidateAndOptimize(reader, model.NewDefaultConfiguration())
	if err != nil {
		t.Fatalf("ReadValidateAndOptimize failed with %s", err)
	}

	if ctx.PageCount != 2 {
		t.Fatalf("Expected 2 pages got %d", ctx.PageCount)
	}
}

func TestNotFoundWhenRequestingNonExistingResource(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	orgId := "someOrg"
	store.RegisterOrganization(context.Background(), &pkg.Organization{Id: orgId})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/resources/0aaax", nil)
	request = withAuthSession(request, orgId)
	handler := ResourceDownload(store, 1*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("Expected %d got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestDeleteResourceFromProjectHandler(t *testing.T) {
	store := pkg.NewDemoStore()

	orgId := store.FirstOrganizationId()

	var projectId string
	for id := range store.Data[orgId].Projects {
		projectId = id
		break
	}

	resourceId := store.Data[orgId].Projects[projectId].ResourceIds[0]

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("DELETE", "/projects/"+projectId+"/"+resourceId, nil)
	request = withAuthSession(request, orgId)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /projects/{projectId}/{resourceId}", RemoveFromProject(store, 1*time.Second))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected code %d got %d", http.StatusOK, recorder.Code)
	}

	for _, id := range store.Data[orgId].Projects[projectId].ResourceIds {
		if id == resourceId {
			t.Fatalf("%s should not be part of the project anymore", resourceId)
		}
	}
}

type failingResourceRemover struct {
	err error
}

func (f *failingResourceRemover) RemoveResource(ctx context.Context, orgId string, projectId string, resourceId string) error {
	return f.err
}

func TestFailingRemover(t *testing.T) {
	remover := failingResourceRemover{
		err: errors.New("Something went wrong"),
	}

	handler := RemoveFromProject(&remover, 1*time.Second)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("DELETE", "/projects/000/111", nil)
	request = withAuthSession(request, "someOrg")
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Wanted code %d got %d", http.StatusInternalServerError, recorder.Code)
	}
}

func TestAddToResourceSubmitForm(t *testing.T) {
	store := pkg.NewDemoStore()

	var inMemStore *pkg.InMemoryStore
	var orgId string
	for id, s := range store.Data {
		inMemStore = s
		orgId = id
		break
	}

	var projectId string
	for id := range inMemStore.Projects {
		projectId = id
		break
	}
	resourceId := inMemStore.Projects[projectId].ResourceIds[0]

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/resources/"+resourceId+"/submit-form", nil)
	request = withAuthSession(request, orgId)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /resources/{id}/submit-form", AddToResourceHandler(store, 1*time.Second))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Wanted %d got %d", http.StatusOK, recorder.Code)
	}
}

func TestAddResourceSubmitFormResourceNotFound(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	orgId := "orgId"
	store.RegisterOrganization(context.Background(), &pkg.Organization{Id: orgId})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/resources/000/submit-form", nil)
	request = withAuthSession(request, orgId)
	mux := http.NewServeMux()
	mux.HandleFunc("/resources/{id}/submit-form", AddToResourceHandler(store, 1*time.Second))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected code %d got %d", http.StatusInternalServerError, recorder.Code)
	}
}

func TestHandleGoogleLoginMissingKey(t *testing.T) {
	opt := sessions.Options{}
	cookie := sessions.NewCookieStore([]byte{})
	handler := RequireSession(cookie, AuthSession, &opt)(HandleGoogleLogin(pkg.NewDefaultConfig().OAuthConfig()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/login", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusInternalServerError, recorder.Code)
	}

	text := recorder.Body.String()
	if !strings.Contains(text, "save session") {
		t.Fatalf("Wanted text to contain 'save session' got '%s'", text)
	}
}

func TestHandleGoogleLogin(t *testing.T) {
	opt := sessions.Options{}
	cookie := sessions.NewCookieStore([]byte("some-random-key"))
	handler := RequireSession(cookie, AuthSession, &opt)(HandleGoogleLogin(pkg.NewDefaultConfig().OAuthConfig()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/login", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusTemporaryRedirect, recorder.Code)
	}
}

func TestInviteLinkAddedToSession(t *testing.T) {
	opt := sessions.Options{}
	cookie := sessions.NewCookieStore([]byte("top-secret"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/login?invite-token=ddaa", nil)
	handler := RequireSession(cookie, AuthSession, &opt)(HandleGoogleLogin(pkg.NewDefaultConfig().OAuthConfig()))
	handler.ServeHTTP(recorder, request)

	session, err := cookie.Get(request, AuthSession)
	if err != nil {
		t.Fatal(err)
	}
	link, ok := session.Values["invite-token"].(string)
	if !ok {
		t.Fatal("Could not convert inviteToken to string")
	}

	if link != "ddaa" {
		t.Fatalf("Wanted 'ddaa' got '%s'", link)
	}
}

func prepareGoogleCallbackRequest(cookie sessions.Store, sessionModifier ...func(s *sessions.Session)) *http.Request {
	stateString := "oauth-state-string"
	formData := url.Values{}
	formData.Set("state", stateString)
	formData.Set("code", "some-code")

	request := httptest.NewRequest("POST", "/auth/callback", strings.NewReader(formData.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	session := utils.Must(cookie.Get(request, AuthSession))

	session.Values[OAuthState] = stateString

	for _, fn := range sessionModifier {
		fn(session)
	}
	request = request.WithContext(context.WithValue(request.Context(), sessionKey, session))
	request.ParseForm()
	return request
}

func TestHandleGoogleLoginCallbackOk(t *testing.T) {
	req := prepareGoogleCallbackRequest(sessions.NewCookieStore([]byte("some-random-key")))
	transport := NewMockTransport()
	store := pkg.NewDemoStore()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), 1*time.Second, "signKey", transport)

	recorder := httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("Wanted %d got %d", http.StatusSeeOther, recorder.Code)
	}

	session, ok := req.Context().Value(sessionKey).(*sessions.Session)
	if !ok {
		t.Fatal("Could not cast into *sessions.Session")
	}

	data, ok := session.Values["role"].([]byte)
	if !ok {
		t.Fatal("Could not cast into bytes")
	}

	var role pkg.UserInfo
	if err := json.Unmarshal(data, &role); err != nil {
		t.Fatalf("Could not unmarshal session content: %s", err)
	}

}

func TestHandleGoogleLoginCallbackInvalidAuthState(t *testing.T) {
	req := prepareGoogleCallbackRequest(sessions.NewCookieStore([]byte("some-random-key")))
	session, ok := req.Context().Value(sessionKey).(*sessions.Session)
	if !ok {
		t.Fatal("Could not interpret value as *sessions.Session")
	}

	session.Values[OAuthState] = "altered-state-string"
	store := pkg.NewMultiOrgInMemoryStore()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, "signKey", nil)

	recorder := httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Wanted %d got %d", http.StatusBadRequest, recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Invalid OAuth state") {
		t.Fatalf("Wanted body to contain 'Invalid OAuth state' got %s", body)
	}
}

func TestOnlyIdPresentForNewUser(t *testing.T) {
	cookieStore := sessions.NewCookieStore([]byte("some-random-key"))
	req := prepareGoogleCallbackRequest(cookieStore)

	store := pkg.NewMultiOrgInMemoryStore()
	transport := NewMockTransport()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, "signKey", transport)

	recorder := httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusFound, recorder.Code)
	}

	resp := recorder.Result()
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Wanted 1 cookie got %d", len(cookies))
	}

	session, err := cookieStore.Get(req, AuthSession)
	if err != nil {
		t.Fatal(err)
	}

	userId, ok := session.Values["userId"].(string)
	if !ok {
		t.Fatalf("Could not get user ID")
	}

	if userId == "" {
		t.Fatalf("User id should not be an emptry string")
	}

}

func TestInternalServerErrorOnCookieSaveFailure(t *testing.T) {
	req := prepareGoogleCallbackRequest(&errorStore{})
	store := pkg.NewDemoStore()
	transport := NewMockTransport()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, "signKey", transport)

	recorder := httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Wanted code '%d' got '%d'", http.StatusInternalServerError, recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "mock save error") {
		t.Fatalf("Wanted body to contain 'mock save error' got %s", body)
	}
}

func TestHandleGoogleLoginCallbackBadRequestOnMissingCode(t *testing.T) {
	req := prepareGoogleCallbackRequest(sessions.NewCookieStore([]byte("some-random-key")))
	req.PostForm.Del("code")
	req.Form = req.PostForm

	encoded := req.PostForm.Encode()
	req.Body = io.NopCloser(strings.NewReader(encoded))
	req.ContentLength = int64(len(encoded))

	store := pkg.NewMultiOrgInMemoryStore()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, "signKey", nil)
	recorder := httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusBadRequest, recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "Code") {
		t.Fatalf("Wanted result to contain 'Code' got %s", body)
	}
}

func TestInternalServerErrorOnCodeExchangeFailure(t *testing.T) {
	req := prepareGoogleCallbackRequest(sessions.NewCookieStore([]byte("some-random-key")))

	transport := NewMockTransport(WithTokenResponse(NewNotFoundResponse()))
	store := pkg.NewMultiOrgInMemoryStore()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, "signKey", transport)

	recorder := httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusInternalServerError, recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "exchange") {
		t.Fatalf("Wanted body to contain 'exchange' got %s", body)
	}
}

func TestInternalServerErrorOnUserRespFailure(t *testing.T) {
	req := prepareGoogleCallbackRequest(sessions.NewCookieStore([]byte("some-random-key")))

	for i, test := range []struct {
		userResp   *http.Response
		code       int
		bodySubstr string
	}{
		{
			userResp:   NewNotFoundResponse(),
			code:       http.StatusNotFound,
			bodySubstr: "getting user info",
		},
		{
			userResp:   NewEmptyResponse(),
			code:       http.StatusInternalServerError,
			bodySubstr: "decoding user info",
		},
	} {
		transport := NewMockTransport(WithUserInfoResponse(test.userResp))
		store := pkg.NewMultiOrgInMemoryStore()
		handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, "signKey", transport)
		recorder := httptest.NewRecorder()
		handler(recorder, req)

		if recorder.Code != test.code {
			t.Fatalf("Test #%d: Wanted '%d' got '%d'", i, http.StatusInternalServerError, recorder.Code)
		}
		body := recorder.Body.String()

		if !strings.Contains(body, test.bodySubstr) {
			t.Fatalf("Test #%d: Wanted body containing '%s' got '%s'", i, test.bodySubstr, body)
		}
	}
}

func TestViewerFromInviteLink(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	inviteClaim := InviteClaim{OrgId: "new-organization"}
	signKey := "top-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, inviteClaim)
	signedToken, err := token.SignedString([]byte(signKey))
	if err != nil {
		t.Fatal(err)
	}

	cookie := sessions.NewCookieStore([]byte(signKey))
	req := prepareGoogleCallbackRequest(cookie, func(s *sessions.Session) {
		s.Values["invite-token"] = signedToken
	})

	transport := NewMockTransport()
	recorder := httptest.NewRecorder()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, signKey, transport)
	handler(recorder, req)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusSeeOther, recorder.Code)
	}

	var user pkg.UserInfo
	for _, u := range store.Users {
		user = u
		break
	}

	// Confirm that the role of the user was registered
	if role, ok := user.Roles["new-organization"]; role != pkg.RoleViewer || !ok {
		t.Fatalf("Expected user to get a role in the provided organization")
	}

	session, err := cookie.Get(req, AuthSession)
	if err != nil {
		t.Fatal(err)
	}

	orgId, ok := session.Values["orgId"].(string)
	if !ok {
		t.Fatalf("Could not convert organization into string")
	}

	if orgId != "new-organization" {
		t.Fatalf("Expected organization id to be set to the newly created organization got %s", orgId)
	}

	roles, ok := session.Values["role"].([]byte)
	if !ok {
		t.Fatalf("Could not convert role into []byte")
	}

	var userSession pkg.UserInfo
	if err := json.Unmarshal(roles, &userSession); err != nil {
		t.Fatal(err)
	}

	sessionRole, ok := userSession.Roles["new-organization"]
	if !ok {
		t.Fatalf("New organizatation not added in the session user info %v", userSession)
	}
	if sessionRole != pkg.RoleViewer {
		t.Fatalf("Session role is not %d got %d", pkg.RoleViewer, sessionRole)
	}
}

func TestBadRequestOnWrongToken(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	inviteClaim := InviteClaim{OrgId: "new-organization"}
	signKey := "top-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, inviteClaim)

	// Sign the token with the wrong key
	signedToken, err := token.SignedString([]byte("another-signKey"))
	if err != nil {
		t.Fatal(err)
	}

	cookie := sessions.NewCookieStore([]byte(signKey))
	req := prepareGoogleCallbackRequest(cookie, func(s *sessions.Session) {
		s.Values["invite-token"] = signedToken
	})

	transport := NewMockTransport()
	recorder := httptest.NewRecorder()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, signKey, transport)
	handler(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusSeeOther, recorder.Code)
	}

	body := recorder.Body.String()

	if !strings.Contains(body, "signature is invalid") {
		t.Fatalf("Expected body to contain 'signature is invalid' got %s", body)
	}
}

func TestInternalServerErrorOnFailingRoleHandling(t *testing.T) {
	store := pkg.FailingRoleStore{
		ErrRegisterRole: errors.New("some un expected error occured"),
	}
	inviteClaim := InviteClaim{OrgId: "new-organization"}
	signKey := "top-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, inviteClaim)
	signedToken, err := token.SignedString([]byte(signKey))
	if err != nil {
		t.Fatal(err)
	}

	cookie := sessions.NewCookieStore([]byte(signKey))
	req := prepareGoogleCallbackRequest(cookie, func(s *sessions.Session) {
		s.Values["invite-token"] = signedToken
	})

	transport := NewMockTransport()
	recorder := httptest.NewRecorder()
	handler := HandleGoogleCallback(&store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, signKey, transport)
	handler(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusInternalServerError, recorder.Code)
	}

	body := recorder.Body.String()

	if !strings.Contains(body, store.ErrRegisterRole.Error()) {
		t.Fatalf("Wanted body to contain '%s' got '%s'", store.ErrRegisterRole.Error(), body)
	}
}

func TestRootHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	RootHandler(recorder, req)
	testutils.AssertEqual(t, recorder.Code, http.StatusOK)
}

func TestOrganizationHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/organizations", nil)
	recorder := httptest.NewRecorder()
	OrganizationsHandler(recorder, req)
	testutils.AssertEqual(t, recorder.Code, http.StatusOK)
}

func TestInviteLinkHandler(t *testing.T) {
	url := "http://myapp.com"
	secret := "top-secret"
	handler := InviteLink(url, secret)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/organizations/1234-431/invite", nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /organizations/{id}/invite", handler)
	mux.ServeHTTP(recorder, request)

	testutils.AssertEqual(t, recorder.Code, http.StatusOK)
	testutils.AssertEqual(t, recorder.Header()["Content-Type"][0], "application/json")

	body := recorder.Body.String()
	testutils.AssertContains(t, body, url, "/login?invite-token=", "invite_link")
}

func TestOrganizationRegisterFormErrors(t *testing.T) {
	for _, test := range []struct {
		body []byte
		code int
		desc string
	}{
		{
			body: bytes.Repeat([]byte("a"), 6*1024), // 6 * 1024 = 6144
			code: http.StatusRequestEntityTooLarge,
			desc: "Body is too large",
		},
		{
			code: http.StatusBadRequest,
			desc: "Name missing in form",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			// Create a test HTTP POST request with the body
			req := httptest.NewRequest(http.MethodPost, "/example", bytes.NewReader(test.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			recorder := httptest.NewRecorder()
			store := pkg.NewMultiOrgInMemoryStore()
			handler := OrganizationRegisterHandler(store, time.Second)
			handler(recorder, req)
			testutils.AssertEqual(t, recorder.Code, test.code)
		})
	}
}

func TestOrganizationHandlerSuccess(t *testing.T) {
	functioningStore := pkg.NewMultiOrgInMemoryStore()
	failStore := pkg.MockIAMStore{
		ErrRegisterRole: errors.New("an error occured"),
	}

	for _, test := range []struct {
		store pkg.IAMStore
		code  int
		desc  string
	}{
		{
			store: functioningStore,
			code:  http.StatusOK,
			desc:  "Ok request",
		},
		{
			store: &failStore,
			code:  http.StatusInternalServerError,
			desc:  "Registering role fails",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			form := url.Values{}
			form.Add("name", "my organization")

			// Create a new HTTP POST request with form data
			req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			cookie := sessions.NewCookieStore([]byte("top-secret"))
			session, err := cookie.Get(req, AuthSession)
			testutils.AssertNil(t, err)

			session.Values["userId"] = "0000-0000"
			ctx := context.WithValue(req.Context(), sessionKey, session)

			recorder := httptest.NewRecorder()
			handler := OrganizationRegisterHandler(test.store, time.Second)
			handler(recorder, req.WithContext(ctx))
			testutils.AssertEqual(t, recorder.Code, test.code)
		})
	}
}

func TestOptionFromSession(t *testing.T) {
	cookie := sessions.NewCookieStore([]byte("top-secret"))
	req := httptest.NewRequest("GET", "/options", nil)
	session, err := cookie.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	userInfo := pkg.UserInfo{
		Id:    "0000-000",
		Roles: map[string]pkg.RoleKind{"111-111": pkg.RoleEditor, "01": pkg.RoleEditor, "21": pkg.RoleEditor},
	}

	store := pkg.NewMultiOrgInMemoryStore()

	// Deliberately make "01" not exist
	store.Organizations = []pkg.Organization{
		{
			Id:   "111-111",
			Name: "First organization",
		},
		{
			Id:   "21",
			Name: "21 organization",
		},
	}

	data, err := json.Marshal(userInfo)
	testutils.AssertNil(t, err)
	session.Values["role"] = data

	handler := OptionsFromSessionHandler(store, time.Second)
	recorder := httptest.NewRecorder()
	ctx := context.WithValue(req.Context(), sessionKey, session)
	handler(recorder, req.WithContext(ctx))
	testutils.AssertEqual(t, recorder.Code, http.StatusOK)

	body := recorder.Body.String()
	testutils.AssertContains(t, body, "111-111", "21", "First organization", "21 organization")

	// Update session with an organization ID. That should have som impact on the order
	re := regexp.MustCompile(`<option\s+value="([^"]+)"`) // Matches the id in option field
	for _, id := range []string{"111-111", "21"} {
		session.Values["orgId"] = id
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		body = recorder.Body.String()

		// Now the ID in the orgId should appear first in the list
		matches := re.FindStringSubmatch(body)
		testutils.AssertEqual(t, matches[1], id)
	}
}

func TestDeleteOrganizationHandler(t *testing.T) {
	cookie := sessions.NewCookieStore([]byte("top-secret"))
	req := httptest.NewRequest("DELETE", "/organizations", nil)

	session, err := cookie.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	store := pkg.NewMultiOrgInMemoryStore()
	store.Organizations = []pkg.Organization{
		{
			Id:   "0000-0000",
			Name: "Org1",
		},
	}

	userInfo := pkg.UserInfo{
		Id:    "1111-111",
		Roles: map[string]pkg.RoleKind{"0000-0000": pkg.RoleAdmin},
	}

	data, err := json.Marshal(userInfo)
	testutils.AssertNil(t, err)

	session.Values["role"] = data
	session.Values["orgId"] = "0000-0000"
	ctx := context.WithValue(req.Context(), sessionKey, session)
	handler := DeleteOrganizationHandler(store, time.Second)

	t.Run("Successful delete", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)

		respCookies := recorder.Result().Cookies()
		testutils.AssertEqual(t, len(respCookies), 1)
		respCookie := respCookies[0]
		req2 := httptest.NewRequest("GET", "/whatever", nil)
		req2.AddCookie(respCookie)
		sessionFromResp, err := cookie.Get(req2, AuthSession)
		testutils.AssertNil(t, err)

		orgId, ok := sessionFromResp.Values["orgId"].(string)
		if !ok {
			t.Fatalf("Could not cast orgId to string")
		}
		testutils.AssertEqual(t, orgId, "")

		jsonData, ok := sessionFromResp.Values["role"].([]byte)
		if !ok {
			t.Fatalf("Could not cast role to []byte")
		}

		var infoFromCookie pkg.UserInfo
		err = json.Unmarshal(jsonData, &infoFromCookie)
		testutils.AssertNil(t, err)
		testutils.AssertEqual(t, len(infoFromCookie.Roles), 0)
	})

	t.Run("Session store failure", func(t *testing.T) {
		session.Values["value"] = make(chan int) // Channel can not be stored in session
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)

		body := recorder.Body.String()
		testutils.AssertContains(t, body, "update session")
	})
}

func TestSessionPersistOnStoreError(t *testing.T) {
	store := pkg.MockIAMStore{
		ErrDeleteOrganization: errors.New("unexpected error"),
	}

	cookie := sessions.NewCookieStore([]byte("top-secret"))
	req := httptest.NewRequest("DELETE", "/organizations", nil)
	session, err := cookie.Get(req, AuthSession)
	testutils.AssertNil(t, err)
	session.Values["orgId"] = "0000-0000"

	ctx := context.WithValue(req.Context(), sessionKey, session)
	handler := DeleteOrganizationHandler(&store, time.Second)
	recorder := httptest.NewRecorder()
	handler(recorder, req.WithContext(ctx))
	testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)

	respCookies := recorder.Result().Cookies()

	// No update in session
	testutils.AssertEqual(t, len(respCookies), 0)

	body := recorder.Body.String()
	testutils.AssertContains(t, body, "delete organization")
}

func TestChosenOrganizationHandlerMissingExistingOrg(t *testing.T) {
	req := httptest.NewRequest("GET", "/endpoint", nil)
	recorder := httptest.NewRecorder()
	ChosenOrganizationSessionHandler(recorder, req)
	testutils.AssertEqual(t, recorder.Code, http.StatusBadRequest)

	body := recorder.Body.String()
	testutils.AssertContains(t, body, "organization", "id")
}

func TestChosenOrganizationSessionHandler(t *testing.T) {
	cookieStore := sessions.NewCookieStore([]byte("top-secret"))

	req := httptest.NewRequest("GET", "/endpoint?existing_org=111", nil)
	session, err := cookieStore.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	ctx := context.WithValue(req.Context(), sessionKey, session)
	t.Run("Successfully add organization id", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ChosenOrganizationSessionHandler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		id, ok := session.Values["orgId"].(string)
		if !ok {
			t.Fatal("Could not convert organization ID to string")
		}
		testutils.AssertEqual(t, id, "111")
	})

	t.Run("Fail to save", func(t *testing.T) {
		session.Values["whatever"] = make(chan int)
		recorder := httptest.NewRecorder()
		ChosenOrganizationSessionHandler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)
		testutils.AssertContains(t, recorder.Body.String(), "save", "session")
	})
}

func TestActiveOrganization(t *testing.T) {
	cookieStore := sessions.NewCookieStore([]byte("top-secret"))

	req := httptest.NewRequest("GET", "/endpoint", nil)
	session, err := cookieStore.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	ctx := context.WithValue(req.Context(), sessionKey, session)
	store := pkg.NewMultiOrgInMemoryStore()
	store.Organizations = []pkg.Organization{
		{
			Id:   "111",
			Name: "Org",
		},
	}
	handler := ActiveOrganization(store, time.Second)

	t.Run("Missing org id", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		testutils.AssertEqual(t, recorder.Body.String(), "No organization")
	})

	t.Run("Empty org id", func(t *testing.T) {
		session.Values["orgId"] = ""
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		testutils.AssertEqual(t, recorder.Body.String(), "No organization")
	})

	t.Run("Valid org id", func(t *testing.T) {
		session.Values["orgId"] = "111"
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		testutils.AssertEqual(t, recorder.Body.String(), "Org")
	})

	t.Run("Invalid org id", func(t *testing.T) {
		session.Values["orgId"] = "112"
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		testutils.AssertEqual(t, recorder.Body.String(), "No organization")
	})
}

func TestPeopleHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/endpoint", nil)
	recorder := httptest.NewRecorder()
	PeoplePage(recorder, req)
	testutils.AssertEqual(t, recorder.Code, http.StatusOK)
}

func TestAllUsers(t *testing.T) {
	cookieStore := sessions.NewCookieStore([]byte("top-secret"))
	req := httptest.NewRequest("GET", "/endpoint", nil)
	session, err := cookieStore.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	store := pkg.NewMultiOrgInMemoryStore()
	store.Users = []pkg.UserInfo{
		{
			Id:    "1000",
			Roles: map[string]pkg.RoleKind{"1000": pkg.RoleViewer},
			Name:  "John",
		},
		{
			Id:   "0000",
			Name: "Peter",
			Roles: map[string]pkg.RoleKind{
				"1000": pkg.RoleAdmin,
				"2000": pkg.RoleEditor,
			}},
	}
	session.Values["role"] = utils.Must(json.Marshal(store.Users[1]))

	handler := AllUsers(store, time.Second)
	ctx := context.WithValue(req.Context(), sessionKey, session)

	t.Run("Test admin OK", func(t *testing.T) {
		session.Values["orgId"] = "1000"
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		body := recorder.Body.String()
		testutils.AssertContains(t, body, "Peter", "John")
	})

	t.Run("Test reader OK", func(t *testing.T) {
		session.Values["orgId"] = "2000"
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		body := recorder.Body.String()
		testutils.AssertContains(t, body, "Peter")
		testutils.AssertNotContains(t, body, "John")
	})

	failingStore := pkg.MockIAMStore{
		ErrUserInOrg:   errors.New("user in organization failed"),
		ErrGetUserInfo: errors.New("get user info error"),
	}

	failingHandler := AllUsers(&failingStore, time.Second)
	t.Run("Test failing admin", func(t *testing.T) {
		session.Values["orgId"] = "1000"
		recorder := httptest.NewRecorder()
		failingHandler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)
	})

	t.Run("Test failing reader", func(t *testing.T) {
		session.Values["orgId"] = "2000"
		recorder := httptest.NewRecorder()
		failingHandler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)
	})

	reqWithQuery := httptest.NewRequest("GET", "/users?name=pe", nil)
	t.Run("Test admin OK with filter", func(t *testing.T) {
		session.Values["orgId"] = "1000"
		recorder := httptest.NewRecorder()
		handler(recorder, reqWithQuery.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		body := recorder.Body.String()
		testutils.AssertContains(t, body, "Peter")
		testutils.AssertNotContains(t, body, "John")
	})
}

func TestAssignRoleHandler(t *testing.T) {
	cookieStore := sessions.NewCookieStore([]byte("top-secret"))
	req := httptest.NewRequest("GET", "/endpoint", nil)
	session, err := cookieStore.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	store := pkg.NewMultiOrgInMemoryStore()
	store.Users = []pkg.UserInfo{
		{
			Id:    "0000-0001",
			Roles: map[string]pkg.RoleKind{"1000-0000": pkg.RoleViewer},
		},
	}
	mux := http.NewServeMux()

	handler := AssignRoleHandler(store, time.Second)
	mux.HandleFunc("POST /organizations/users/{id}/role", handler)

	session.Values["userId"] = "0000-0000"
	session.Values["orgId"] = "1000-0000"
	ctx := context.WithValue(req.Context(), sessionKey, session)

	t.Run("test can not alter self", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/organizations/users/0000-0000/role", nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusForbidden)
	})

	t.Run("test fail on large body", func(t *testing.T) {
		buf := bytes.NewReader(bytes.Repeat([]byte("a"), 6000))
		req := httptest.NewRequest("POST", "/organizations/users/0000-0001/role", buf)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusRequestEntityTooLarge)
	})

	t.Run("test bad request on integer conversion error", func(t *testing.T) {
		form := url.Values{}
		form.Set("role", "not an int")

		reader := bytes.NewReader([]byte(form.Encode()))
		req := httptest.NewRequest("POST", "/organizations/users/0000-0001/role", reader)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusBadRequest)
		body := recorder.Body.String()
		testutils.AssertContains(t, body, "invalid")
	})

	for _, test := range []struct {
		role     int
		wantRole pkg.RoleKind
		desc     string
	}{
		{
			role:     1,
			wantRole: 1,
			desc:     "Register writer role",
		},
		{
			role:     42,
			wantRole: 0,
			desc:     "Role larger than admin, should set to reader",
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			form := url.Values{}
			form.Set("role", strconv.Itoa(test.role))
			reader := bytes.NewReader([]byte(form.Encode()))
			req := httptest.NewRequest("POST", "/organizations/users/0000-0001/role", reader)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req.WithContext(ctx))
			testutils.AssertEqual(t, recorder.Code, http.StatusOK)
			testutils.AssertEqual(t, store.Users[0].Roles["1000-0000"], test.wantRole)
		})
	}

	failingStore := pkg.MockIAMStore{
		ErrRegisterRole: errors.New("something went wrong"),
	}

	failingHandler := AssignRoleHandler(&failingStore, time.Second)
	t.Run("test registration fails", func(t *testing.T) {
		form := url.Values{}
		form.Set("role", "1")
		reader := bytes.NewReader([]byte(form.Encode()))
		req := httptest.NewRequest("POST", "/organizations/users/0000-0001/role", reader)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		failingHandler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)
		body := recorder.Body.String()
		testutils.AssertContains(t, body, "register new role")

	})
}

func TestRegisterRecipent(t *testing.T) {
	cookieStore := sessions.NewCookieStore([]byte("top-secret"))

	form := url.Values{}
	form.Set("name", "john")
	form.Set("email", "john@gmail.com")
	form.Set("group", "tenor")

	formReader := bytes.NewReader([]byte(form.Encode()))
	req := httptest.NewRequest("POST", "/endpoint", formReader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	session, err := cookieStore.Get(req, AuthSession)
	session.Values["orgId"] = "0000-0000"
	testutils.AssertNil(t, err)

	store := pkg.NewMultiOrgInMemoryStore()
	ctx := context.WithValue(req.Context(), sessionKey, session)

	handler := RegisterRecipent(store, time.Second)
	t.Run("test register user", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		testutils.AssertEqual(t, len(store.Users), 1)
		testutils.AssertEqual(t, store.Users[0].Roles["0000-0000"], pkg.RoleViewer)
	})

	t.Run("test fail on too large", func(t *testing.T) {
		data := bytes.Repeat([]byte("a"), 6000)
		buf := bytes.NewBuffer(data)
		largeReq := httptest.NewRequest("POST", "/endpoint", buf)
		largeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		recorder := httptest.NewRecorder()
		handler(recorder, largeReq.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusRequestEntityTooLarge)
	})

	failing := pkg.MockIAMStore{
		ErrRegisterUser: errors.New("something went wrong"),
	}

	failingHandler := RegisterRecipent(&failing, time.Second)
	t.Run("test register user fails", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		failingHandler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)
		testutils.AssertContains(t, recorder.Body.String(), "register recipent")
	})
}

func TestDeleteRole(t *testing.T) {
	req := httptest.NewRequest("DELETE", "/organizations/users/1000", nil)
	cookieStore := sessions.NewCookieStore([]byte("top-secret"))
	session, err := cookieStore.Get(req, AuthSession)
	session.Values["userId"] = "2000"
	session.Values["orgId"] = "0000-0000"
	testutils.AssertNil(t, err)

	store := pkg.NewMultiOrgInMemoryStore()
	store.Users = []pkg.UserInfo{
		{
			Id:    "1000",
			Roles: map[string]pkg.RoleKind{"0000-0000": pkg.RoleViewer},
		},
	}
	ctx := context.WithValue(req.Context(), sessionKey, session)

	handler := DeleteUserFromOrg(store, time.Second)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /organizations/users/{id}", handler)

	t.Run("test not possible to delete self", func(t *testing.T) {
		selfDelete := httptest.NewRequest("DELETE", "/organizations/users/2000", nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, selfDelete.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusForbidden)
	})

	t.Run("test delete other user", func(t *testing.T) {
		orgId := "0000-0000"
		recorder := httptest.NewRecorder()
		_, hasRole := store.Users[0].Roles[orgId]
		testutils.AssertEqual(t, hasRole, true)

		mux.ServeHTTP(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		_, hasRole = store.Users[0].Roles[orgId]
		testutils.AssertEqual(t, hasRole, false)
	})

	failingStore := pkg.MockIAMStore{
		ErrDeleteUserRole: errors.New("unexpected error"),
	}
	failingHandler := DeleteUserFromOrg(&failingStore, time.Second)
	t.Run("deletion fails", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		failingHandler(recorder, req.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusInternalServerError)
		testutils.AssertContains(t, recorder.Body.String(), "delete role")
	})
}

func TestGroupHandler(t *testing.T) {
	cookieStore := sessions.NewCookieStore([]byte("top-secret"))

	form := url.Values{}
	form.Set("group", "Alto")
	req := httptest.NewRequest("POST", "/organizations/users/0000/groups", bytes.NewReader([]byte(form.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	session, err := cookieStore.Get(req, AuthSession)
	testutils.AssertNil(t, err)
	readerOrg := "0000-0000"
	adminOrg := "1000-0000"
	userInfo := pkg.UserInfo{
		Id: "0000",
		Roles: map[string]pkg.RoleKind{
			readerOrg: pkg.RoleViewer,
			adminOrg:  pkg.RoleAdmin,
		},
		Groups: make(map[string][]string),
	}

	session.Values["userId"] = userInfo.Id
	session.Values["role"] = utils.Must(json.Marshal(userInfo))
	session.Values["orgId"] = readerOrg

	store := pkg.NewMultiOrgInMemoryStore()
	store.Users = []pkg.UserInfo{userInfo, {Id: "1000", Groups: make(map[string][]string)}}

	ctx := context.WithValue(req.Context(), sessionKey, session)
	handler := GroupHandler(store, time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/users/{id}/groups", handler)
	mux.HandleFunc("DELETE /organizations/users/{id}/groups", handler)

	t.Run("test error on too large body", func(t *testing.T) {
		data := bytes.Repeat([]byte("a"), 6000)
		largeReq := httptest.NewRequest("POST", "/organizations/users/0000/groups", bytes.NewReader(data))
		largeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, largeReq.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusRequestEntityTooLarge)
	})

	t.Run("test reader can edit own roles", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req.WithContext(ctx))

		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		groups := store.Users[0].Groups[readerOrg]
		testutils.AssertEqual(t, slices.Contains(groups, "Alto"), true)
	})

	t.Run("test reader can not edit others roles", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/organizations/users/1000/groups", bytes.NewReader([]byte(form.Encode())))
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, r.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusUnauthorized)
	})

	t.Run("test admin can edit other roles", func(t *testing.T) {
		session.Values["orgId"] = adminOrg
		formBody := []byte(form.Encode())
		r := httptest.NewRequest("POST", "/organizations/users/1000/groups", bytes.NewReader(formBody))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, r.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
		testutils.AssertEqual(t, slices.Contains(store.Users[1].Groups[adminOrg], "Alto"), true)

		delReq := httptest.NewRequest("DELETE", "/organizations/users/1000/groups?group=Alto", nil)
		delRec := httptest.NewRecorder()
		mux.ServeHTTP(delRec, delReq.WithContext(ctx))
		testutils.AssertEqual(t, delRec.Code, http.StatusOK)
		testutils.AssertEqual(t, len(store.Users[1].Groups[adminOrg]), 0)
	})

	failingStore := pkg.MockIAMStore{
		ErrRegisterGroup: errors.New("something went wrong"),
		ErrRemoveGroup:   errors.New("something went wrong"),
	}

	failingHandler := GroupHandler(&failingStore, time.Second)
	t.Run("test internal server error on failing writes", func(t *testing.T) {
		session.Values["orgId"] = adminOrg

		for _, method := range []string{"POST", "DELETE"} {
			req := httptest.NewRequest(method, "/organizations", nil)
			rec := httptest.NewRecorder()
			failingHandler(rec, req.WithContext(ctx))
			testutils.AssertEqual(t, rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestLoggedIn(t *testing.T) {
	store := sessions.NewCookieStore([]byte("top-secret"))
	req := httptest.NewRequest("GET", "/endpoint", nil)
	session, err := store.Get(req, AuthSession)
	testutils.AssertNil(t, err)

	ctx := context.WithValue(req.Context(), sessionKey, session)
	t.Run("Not logged in", func(t *testing.T) {
		rec := httptest.NewRecorder()
		LoggedIn(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusOK)
		testutils.AssertContains(t, rec.Body.String(), "Sign in")
	})

	t.Run("Logged in", func(t *testing.T) {
		session.Values["userId"] = "1234s"
		rec := httptest.NewRecorder()
		LoggedIn(rec, req.WithContext(ctx))
		testutils.AssertEqual(t, rec.Code, http.StatusOK)
		testutils.AssertContains(t, rec.Body.String(), "Signed in")
	})
}

func TestSendEmail(t *testing.T) {
	config := pkg.NewDefaultConfig()
	config.SmtpConfig.SendFn = pkg.NoOpSendFunc
	store := pkg.NewDemoStore()
	handler := SendEmail(store, config)
	orgId := store.FirstOrganizationId()

	// Add all users to the "part" group
	for i := range store.Users {
		store.Users[i].Groups[orgId] = []string{"part"}
	}

	seen := map[string]struct{}{}
	form := url.Values{}
	for r := range store.Data[orgId].Data {
		splitted := strings.Split(r, "/")
		path := orgId + "/" + strings.Join(splitted[:len(splitted)-1], "/")
		if _, ok := seen[path]; !ok {
			form.Add("resourceId", path)
			seen[path] = struct{}{}
		}
	}

	body := form.Encode()
	cookieStore := sessions.NewCookieStore([]byte("top-secret"))

	validReq := httptest.NewRequest("GET", "/email", bytes.NewBufferString(body))
	validReq.Header.Set("Content-Type", "application/json")

	session, err := cookieStore.Get(validReq, AuthSession)
	testutils.AssertNil(t, err)
	session.Values["orgId"] = orgId
	ctx := context.WithValue(validReq.Context(), sessionKey, session)

	t.Run("valid email", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler(recorder, validReq.WithContext(ctx))
		testutils.AssertEqual(t, recorder.Code, http.StatusOK)
	})
}

func TestSendEmailGetUserFails(t *testing.T) {
	collector := pkg.FailingEmailDataCollector{
		ErrUsersInOrg: errors.New("could not call users"),
	}

	cookieStore := sessions.NewCookieStore([]byte("top-secret"))
	form := url.Values{}
	form.Add("resourceId", "resource")

	req := httptest.NewRequest("GET", "/email", bytes.NewBufferString(form.Encode()))
	session, err := cookieStore.Get(req, AuthSession)
	testutils.AssertNil(t, err)
	session.Values["orgId"] = "some-org-id"
	ctx := context.WithValue(req.Context(), sessionKey, session)

	handler := SendEmail(&collector, pkg.NewDefaultConfig())

	rec := httptest.NewRecorder()
	handler(rec, req.WithContext(ctx))
	testutils.AssertEqual(t, rec.Code, http.StatusInternalServerError)
	testutils.AssertContains(t, rec.Body.String(), collector.ErrUsersInOrg.Error())
}

func TestEmailErrorOnTooLargeForm(t *testing.T) {
	store := pkg.NewMultiOrgInMemoryStore()
	content := bytes.Repeat([]byte("a"), 40000)
	form := url.Values{}
	form.Add("ids", string(content))

	body := bytes.NewBufferString(form.Encode())
	req := httptest.NewRequest("POST", "/email", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	session := sessions.Session{
		Values: make(map[any]any),
	}
	session.Values["orgId"] = "orgId"

	ctx := context.WithValue(req.Context(), sessionKey, &session)
	handler := SendEmail(store, pkg.NewDefaultConfig())
	handler(rec, req.WithContext(ctx))
	testutils.AssertEqual(t, rec.Code, http.StatusRequestEntityTooLarge)
}

func TestEmailFetchError(t *testing.T) {
	collector := pkg.FailingEmailDataCollector{
		ErrItemGetter: errors.New("something went wrong"),
		ResourceNames: []string{"part1"},
		Users: []pkg.UserInfo{{Id: "userId", Groups: map[string][]string{
			"some-org-id": {"part"},
		}}},
	}

	form := url.Values{}
	form.Add("resourceId", "resource1")
	body := form.Encode()

	session := sessions.Session{
		Values: make(map[any]any),
	}
	session.Values["orgId"] = "some-org-id"

	req := httptest.NewRequest("POST", "/email", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), sessionKey, &session)

	handler := SendEmail(&collector, pkg.NewDefaultConfig())
	handler(rec, req.WithContext(ctx))

	testutils.AssertEqual(t, rec.Code, http.StatusInternalServerError)
}
