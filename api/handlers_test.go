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

	handler := SubmitHandler(pkg.NewInMemoryStore(), 10*time.Second, 10)
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

func TestSubmitHandlerValidRequest(t *testing.T) {
	inMemStore := pkg.NewInMemoryStore()
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := validMultipartForm()
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

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
	content := inMemStore.Data["brandenburgconcertono3_johansebastianbach.zip"]

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
	inMemStore := pkg.NewInMemoryStore()
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
	inMemStore := pkg.NewInMemoryStore()
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
	inMemStore := pkg.NewInMemoryStore()
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
	inMemStore := pkg.NewInMemoryStore()
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
	inMemStore := pkg.NewInMemoryStore()
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
	inMemStore := pkg.NewInMemoryStore()
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
	inMemStore := pkg.NewInMemoryStore()
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

func (f *failingSubmitter) Submit(ctx context.Context, meta *pkg.MetaData, r io.Reader) error {
	return f.err
}

func TestSubmitHandlerStoreErrors(t *testing.T) {
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := validMultipartForm()
	request := httptest.NewRequest("POST", "/resources", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	handler := SubmitHandler(&failingSubmitter{err: errors.New("what??")}, 10*time.Second, 10)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code 500, got %d", recorder.Code)
		return
	}
}

func TestEntityTooLargeWhenUploadIsTooLarge(t *testing.T) {
	inMemStore := pkg.NewInMemoryStore()
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

		handler := OverviewSearchHandler(pkg.NewDemoStore(), 10*time.Second)
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

func (f *failingFetcher) MetaByPattern(ctx context.Context, pattern *pkg.MetaData) ([]pkg.MetaData, error) {
	return nil, f.err
}

func TestInternalServerErrorOnFailure(t *testing.T) {
	expectedError := errors.New("fetch error")
	recorder := httptest.NewRecorder()

	request := httptest.NewRequest("GET", "/overview/search?resource-filter=flute", nil)
	handler := OverviewSearchHandler(&failingFetcher{err: expectedError}, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code 500, got %d", recorder.Code)
		return
	}
}

func TestSearchProjectHandler(t *testing.T) {
	inMemStore := pkg.NewInMemoryStore()
	inMemStore.Projects["test_project"] = pkg.Project{
		Name:        "Test Project",
		ResourceIds: []string{"resource1", "resource2"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects/names?projectQuery=test", nil)

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

func (f *failingProjectByNamer) ProjectsByName(ctx context.Context, name string) ([]pkg.Project, error) {
	return nil, f.err
}

func TestSearchProjectHandlerInternelServerErrorOnFailure(t *testing.T) {
	expectedError := errors.New("fetch error")
	recorder := httptest.NewRecorder()

	request := httptest.NewRequest("GET", "/projects/names?projectQuery=test", nil)
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
	inMemStore := pkg.NewInMemoryStore()
	recorder := httptest.NewRecorder()

	form := url.Values{}
	form.Set("projectQuery", "Test Project")
	request := httptest.NewRequest("POST", "/add-to-project", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	handler := ProjectSubmitHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
		return
	}

	if len(inMemStore.Projects) != 1 {
		t.Errorf("Expected 1 project in store, got %d", len(inMemStore.Projects))
		return
	}

	if inMemStore.Projects["testproject"].Name != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got '%s'", inMemStore.Projects["test_project"].Name)
	}
}

func TestBadRequestOnMissingName(t *testing.T) {
	inMemStore := pkg.NewInMemoryStore()
	recorder := httptest.NewRecorder()

	form := url.Values{}
	request := httptest.NewRequest("POST", "/add-to-project", strings.NewReader(form.Encode()))
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

func (f *failingProjectSubmitter) SubmitProject(ctx context.Context, project *pkg.Project) error {
	return f.err
}

func TestInternaltServerErrorOnProjectSubmitFailure(t *testing.T) {
	expectedError := errors.New("submit error")
	recorder := httptest.NewRecorder()

	inMemStore := &failingProjectSubmitter{err: expectedError}
	form := url.Values{}
	form.Set("projectQuery", "Test Project")
	request := httptest.NewRequest("POST", "/add-to-project", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
	request := httptest.NewRequest("GET", "/add-to-project?bad=%ZZ", nil)

	inMemStore := pkg.NewInMemoryStore()
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

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects/info?projectQuery=test", nil)

	handler := SearchProjectListHandler(inMemStore, 10*time.Second)
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

	handler := ProjectByIdHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

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

func TestProjectByIdBadRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/projects", nil)

	inMemStore := pkg.NewInMemoryStore()
	handler := ProjectByIdHandler(inMemStore, 10*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedError := "Project ID is required"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedError, recorder.Body.String())
	}
}

type failingProjectByIdFetcher struct {
	projectErr error
	metaErr    error
}

func (f *failingProjectByIdFetcher) ProjectById(ctx context.Context, id string) (*pkg.Project, error) {
	return &pkg.Project{Name: "Concert No. 1", ResourceIds: []string{"id1"}}, f.projectErr
}
func (f *failingProjectByIdFetcher) MetaById(ctx context.Context, id string) (*pkg.MetaData, error) {
	return nil, f.metaErr
}

func TestProjectByIdInternalServerError(t *testing.T) {
	expectedError := errors.New("fetch error")
	recorder := httptest.NewRecorder()

	inMemStore := &failingProjectByIdFetcher{projectErr: expectedError}
	request := httptest.NewRequest("GET", "/projects/test_project", nil)
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
	config := pkg.NewDefaultConfig()
	mux := Setup(pkg.NewDemoStore(), config)

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
	id := store.Metadata[0].ResourceId()

	request := httptest.NewRequest("GET", "/resources/"+id+"/content", nil)

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
	resourceId := store.Metadata[0].ResourceId()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/resources/"+resourceId, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /resources/{id}", ResourceDownload(store, 1*time.Second))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected code %d got %d", http.StatusOK, recorder.Code)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/zip" {
		t.Fatalf("Expected content type'application/zip'  got %s", contentType)
	}

	resourceName := store.Metadata[0].ResourceName()
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
	resourceId := store.Metadata[0].ResourceId()
	file := "Part2.pdf"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", fmt.Sprintf("/resources/%s?file=%s", resourceId, file), nil)

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
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/resources/0aaax", nil)
	store := pkg.NewInMemoryStore()
	handler := ResourceDownload(store, 1*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("Expected %d got %d", http.StatusNotFound, recorder.Code)
	}
}

type failingResourceGetter struct {
	err error
}

func (f *failingResourceGetter) MetaById(ctx context.Context, id string) (*pkg.MetaData, error) {
	return &pkg.MetaData{}, f.err
}

func (f *failingResourceGetter) Resource(ctx context.Context, name string) (io.Reader, error) {
	return bytes.NewBuffer([]byte{}), f.err
}

func TestInternalServerErrorOnGenericFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/resources/0aaax", nil)
	getter := failingResourceGetter{
		err: errors.New("some generic error"),
	}

	handler := ResourceDownload(&getter, 1*time.Second)
	handler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("Expected %d got %d", http.StatusInternalServerError, recorder.Code)
	}

}
