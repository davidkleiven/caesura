package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/utils"
	"github.com/gorilla/sessions"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestRootHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)
	RootHandler(recorder, request)

	if recorder.Code != 200 {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
	}

	if !strings.Contains(recorder.Body.String(), "Caesura") {
		t.Error("Expected response body to contain 'Caesura'")
	}

}

func TestInstrumentSearchHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/search?token=flute", nil)
	InstrumentSearchHandler(recorder, request)

	if recorder.Code != 200 {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if recorder.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/plain; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
		return
	}

	if !strings.Contains(recorder.Body.String(), "Flute") {
		t.Error("Expected response body to contain 'Flute'")
		return
	}

	if strings.Contains(recorder.Body.String(), "Trumpet") {
		t.Error("Expected response body to not contain 'Trumpet'")
		return
	}

}

func TestChoiceHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/choice?item=flute", nil)
	ChoiceHandler(recorder, request)

	if recorder.Code != 200 {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	expectedResponse := "flute<input type=\"text\" placeholder=\"Enter part number\" id=\"part-number\"/>"
	if recorder.Body.String() != expectedResponse {
		t.Errorf("Expected response body to be '%s', got '%s'", expectedResponse, recorder.Body.String())
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
			t.Errorf("Expected status code 200, got %d", recorder.Code)
			return
		}

		if recorder.Body.String() != test.expected {
			t.Errorf("Expected response body to be '%s', got '%s'", test.expected, recorder.Body.String())
			return
		}
	}
}

func TestDeleteModeWhenFormNotPopulated(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/delete-mode", strings.NewReader("bad=%ZZ")) // malformed encoding
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	DeleteMode(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedError := "Failed to parse form"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
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
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedError := "Failed to parse form"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
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

	userRole := pkg.UserRole{
		UserId: "0000-0000",
		Roles: map[string]pkg.RoleKind{
			orgId: pkg.RoleAdmin,
		},
	}

	data, err := json.Marshal(userRole)
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
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	expectedResponse := "File uploaded successfully"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}

	if len(inMemStore.Data) != 1 {
		t.Errorf("Expected 1 file in store, got %d", len(inMemStore.Data))
		return
	}

	// Check content in the store
	content := inMemStore.Data["orgId"].Data["brandenburgconcertono3_johansebastianbach.zip"]

	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Errorf("Failed to read zip file: %v", err)
		return
	}
	if len(zipReader.File) != 2 {
		t.Errorf("Expected 2 files in zip, got %d", len(zipReader.File))
		return
	}
}

func TestSubmitHandlerInvalidJson(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	var multipartBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBuffer)
	multipartWriter.CreateFormField("filename.pdf")
	contentWriter, err := multipartWriter.CreateFormFile("document", "filename.pdf")
	if err != nil {
		t.Error(err)
		return
	}
	pkg.CreateNPagePdf(contentWriter, 10)

	// Invalid JSON for assignments
	assignments := "invalid json"
	assignmentWriter, err := multipartWriter.CreateFormField("assignments")
	if err != nil {
		t.Error(err)
		return
	}
	assignmentWriter.Write([]byte(assignments))
	if err := multipartWriter.Close(); err != nil {
		t.Error(err)
		return
	}

	request := httptest.NewRequest("POST", "/resources", &multipartBuffer)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedError := "Failed to parse assignments"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestSubmitFormWithoutDocument(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	var multipartBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBuffer)
	if err := multipartWriter.Close(); err != nil {
		t.Error(err)
		return
	}

	request := httptest.NewRequest("POST", "/resources", &multipartBuffer)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedError := "Failed to retrieve file from form"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestSubmitNonPdfFileAsDocument(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withInvalidPdf, withAssignments, withMetaData)

	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code 500, got %d", recorder.Code)
		return
	}

	expectedError := "Failed to split PDF"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
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
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedError := "No assignments provided"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
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
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedError := "No metadata provided"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
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
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedError := "Failed to parse metadata"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestSubmitWithEmptyMetaData(t *testing.T) {
	inMemStore := pkg.NewMultiOrgInMemoryStore()
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withPdf, withAssignments, withEmptyMetaData)
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	handler := SubmitHandler(inMemStore, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedResponse := "Filename is empty."
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}

type failingSubmitter struct {
	err error
}

func (f *failingSubmitter) Submit(ctx context.Context, orgId string, meta *pkg.MetaData, r io.Reader) error {
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
		t.Errorf("Expected status code 500, got %d", recorder.Code)
		return
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

func TestOverHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/overview", nil)

	OverviewHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
		return
	}

	if !strings.Contains(recorder.Body.String(), "Title") {
		t.Error("Expected response body to contain 'Title'")
		return
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
			t.Errorf("Expected status code 200, got %d", recorder.Code)
			return
		}

		if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
			t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
			return
		}

		numRows := strings.Count(recorder.Body.String(), "<tr id=\"row")
		if numRows != test.expectedCount {
			t.Errorf("Expected %d rows in response, got %d", test.expectedCount, numRows)
			return
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
		t.Errorf("Expected status code 500, got %d", recorder.Code)
		return
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
	inMemStore.Data["org1"] = *store

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects/names?projectQuery=test", nil)
	request = withAuthSession(request, "org1")

	handler := SearchProjectHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
		return
	}

	if !strings.Contains(recorder.Body.String(), "Test Project") {
		t.Error("Expected response body to contain 'Test Project'")
		return
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
		t.Errorf("Expected status code 500, got %d", recorder.Code)
		return
	}

	expectedResponse := "Failed to fetch project"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}

func TestProjectSelectorModalHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/overview/project-selector", nil)

	ProjectSelectorModalHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
		return
	}

	if !strings.Contains(recorder.Body.String(), "Confirm") {
		t.Error("Expected response body to contain 'Confirm'")
		return
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
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if len(inMemStore.Data[orgId].Projects) != 1 {
		t.Errorf("Expected 1 project in store, got %d", len(inMemStore.Data[orgId].Projects))
		return
	}

	if inMemStore.Data[orgId].Projects["testproject"].Name != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got '%s'", inMemStore.Data[orgId].Projects["test_project"].Name)
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
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
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
		t.Errorf("Expected status code 500, got %d", recorder.Code)
		return
	}

	expectedResponse := "Failed to submit project"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}

func TestBadRequestWhenWrongApplicationType(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects?bad=%ZZ", nil)

	inMemStore := pkg.NewMultiOrgInMemoryStore()
	handler := ProjectSubmitHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedError := "Failed to parse form"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

func TestProjectQueryInputHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/project-query-input?item=Test%20Project", nil)

	ProjectQueryInputHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
		return
	}

	if !strings.Contains(recorder.Body.String(), "Test Project") {
		t.Error("Expected response body to contain 'Test Project'")
		return
	}
}

func TestJsHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/js/pdf-viewer.js", nil)

	JsHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if recorder.Header().Get("Content-Type") != "application/javascript; charset=utf-8" {
		t.Errorf("Expected Content-Type 'application/javascript; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
		return
	}
}

func TestProjectHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects", nil)

	ProjectHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
		return
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
	multiStore.Data["org1"] = *inMemStore

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects/info?projectQuery=test", nil)
	request = withAuthSession(request, "org1")

	handler := SearchProjectListHandler(multiStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
		return
	}

	if !strings.Contains(recorder.Body.String(), "Test Project") {
		t.Error("Expected response body to contain 'Test Project'")
		return
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
		t.Errorf("Expected status code 500, got %d", recorder.Code)
		return
	}

	expectedResponse := "Failed to fetch projects"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
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
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got '%s'", recorder.Header().Get("Content-Type"))
		return
	}

	if !strings.Contains(recorder.Body.String(), "Demo Project 1") {
		t.Error("Expected response body to contain 'Demo Project 1'")
		return
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
		t.Errorf("Expected status code 500, got %d", recorder.Code)
		return
	}

	expectedResponse := "Failed to fetch project"
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
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
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if !strings.Contains(recorder.Body.String(), "Concert No. 1") {
		t.Error("Expected response body to contain 'Concert No. 1'")
		return
	}
}

func TestSetup(t *testing.T) {
	store := sessions.NewCookieStore([]byte("some-random-key"))
	config := pkg.NewDefaultConfig()
	mux := Setup(pkg.NewDemoStore(), config, store)

	req, _ := http.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
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
		t.Errorf("Expected return code '200' got %d", recorder.Code)
	}

	tokens := []string{"Part0", "Part2", "Part3", "Part4"}
	body := recorder.Body.String()
	for i, token := range tokens {
		if !strings.Contains(body, token) {
			t.Fatalf("Test #%d: expected %s to be part of\n%s\n", i, token, body)
		}
	}
}

func TestResourceContentInternalServerErrorOnGenericError(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/content/0aab", nil)
	request = withAuthSession(request, "someOrg")
	getter := failingResourceGetter{
		err: errors.New("something went wrong"),
	}

	handler := ResourceContentByIdHandler(&getter, 1*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected %d got %d", http.StatusInternalServerError, recorder.Code)
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

	resourceName := store.Data[orgId].Metadata[0].ResourceName()
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

type failingResourceGetter struct {
	err error
}

func (f *failingResourceGetter) MetaById(ctx context.Context, orgId, id string) (*pkg.MetaData, error) {
	return &pkg.MetaData{}, f.err
}

func (f *failingResourceGetter) Resource(ctx context.Context, orgId, name string) (io.Reader, error) {
	return bytes.NewBuffer([]byte{}), f.err
}

func TestInternalServerErrorOnGenericFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/resources/0aaax", nil)
	request = withAuthSession(request, "someOrg")
	getter := failingResourceGetter{
		err: errors.New("some generic error"),
	}

	handler := ResourceDownload(&getter, 1*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected %d got %d", http.StatusInternalServerError, recorder.Code)
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

	var inMemStore pkg.InMemoryStore
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
	cookie := sessions.NewCookieStore([]byte{})
	handler := RequireSession(cookie, AuthSession)(HandleGoogleLogin(pkg.NewDefaultConfig().OAuthConfig()))

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
	cookie := sessions.NewCookieStore([]byte("some-random-key"))
	handler := RequireSession(cookie, AuthSession)(HandleGoogleLogin(pkg.NewDefaultConfig().OAuthConfig()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/login", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusTemporaryRedirect, recorder.Code)
	}
}

func prepareGoogleCallbackRequest(cookie sessions.Store) *http.Request {
	stateString := "oauth-state-string"
	formData := url.Values{}
	formData.Set("state", stateString)
	formData.Set("code", "some-code")

	request := httptest.NewRequest("POST", "/auth/callback", strings.NewReader(formData.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	session := utils.Must(cookie.Get(request, AuthSession))

	session.Values[OAuthState] = stateString
	request = request.WithContext(context.WithValue(request.Context(), sessionKey, session))
	request.ParseForm()
	return request
}

func TestHandleGoogleLoginCallbackOk(t *testing.T) {
	req := prepareGoogleCallbackRequest(sessions.NewCookieStore([]byte("some-random-key")))
	transport := NewMockTransport()
	store := pkg.NewDemoStore()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), 1*time.Second, transport)

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

	var role pkg.UserRole
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
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, nil)

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

func TestNotFoundErrorOnUnkownUserId(t *testing.T) {
	req := prepareGoogleCallbackRequest(sessions.NewCookieStore([]byte("some-random-key")))

	store := pkg.NewMultiOrgInMemoryStore()
	transport := NewMockTransport()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, transport)

	recorder := httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("Wanted '%d' got '%d'", http.StatusFound, recorder.Code)
	}

	text := recorder.Body.String()
	if !strings.Contains(text, "retrieving user") {
		t.Fatalf("Wanted body to contain'retrieving user' got %s", text)
	}
}

func TestInternalServerErrorOnCookieSaveFailure(t *testing.T) {
	req := prepareGoogleCallbackRequest(&errorStore{})
	store := pkg.NewDemoStore()
	transport := NewMockTransport()
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, transport)

	recorder := httptest.NewRecorder()
	handler(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Wanted code '%d' got '%d'", http.StatusInternalServerError, recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "save user role") {
		t.Fatalf("Wanted body to contain 'save user role' got %s", body)
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
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, nil)
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
	handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, transport)

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
		handler := HandleGoogleCallback(store, pkg.NewDefaultConfig().OAuthConfig(), time.Second, transport)
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
