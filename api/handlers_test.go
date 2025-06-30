package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/davidkleiven/caesura/pkg"
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
	request := httptest.NewRequest("GET", "/choice?instrument=flute", nil)
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

func TestSubmitBadRequestHandler(t *testing.T) {
	store := &StoreManager{Store: pkg.NewInMemoryStore()}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/submit", nil)
	request.Header.Set("Content-Type", "multipart/form-data")

	store.SubmitHandler(recorder, request)

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
	metaData := MetaData{
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
	storeMng := &StoreManager{Store: inMemStore}
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := validMultipartForm()
	request := httptest.NewRequest("POST", "/submit", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	storeMng.SubmitHandler(recorder, request)

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
	storeMng := &StoreManager{Store: inMemStore}
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

	request := httptest.NewRequest("POST", "/submit", &multipartBuffer)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	storeMng.SubmitHandler(recorder, request)

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
	storeMng := &StoreManager{Store: inMemStore}
	recorder := httptest.NewRecorder()

	var multipartBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBuffer)
	if err := multipartWriter.Close(); err != nil {
		t.Error(err)
		return
	}

	request := httptest.NewRequest("POST", "/submit", &multipartBuffer)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	storeMng.SubmitHandler(recorder, request)

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
	storeMng := &StoreManager{Store: inMemStore}
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withInvalidPdf, withAssignments, withMetaData)

	request := httptest.NewRequest("POST", "/submit", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	storeMng.SubmitHandler(recorder, request)

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
	storeMng := &StoreManager{Store: inMemStore}
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withPdf, withMetaData)
	request := httptest.NewRequest("POST", "/submit", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	storeMng.SubmitHandler(recorder, request)

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
	storeMng := &StoreManager{Store: inMemStore}
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withPdf, withAssignments)
	request := httptest.NewRequest("POST", "/submit", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	storeMng.SubmitHandler(recorder, request)

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
	storeMng := &StoreManager{Store: inMemStore}
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withPdf, withAssignments, withInvalidMetaData)
	request := httptest.NewRequest("POST", "/submit", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	storeMng.SubmitHandler(recorder, request)

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
	storeMng := &StoreManager{Store: inMemStore}
	recorder := httptest.NewRecorder()

	multipartBuffer, contentType := multipartForm(withPdf, withAssignments, withEmptyMetaData)
	request := httptest.NewRequest("POST", "/submit", multipartBuffer)
	request.Header.Set("Content-Type", contentType)

	storeMng.SubmitHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", recorder.Code)
		return
	}

	expectedResponse := "Filename is empty."
	if !strings.Contains(recorder.Body.String(), expectedResponse) {
		t.Errorf("Expected response body to contain '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}

func TestMetaDataString(t *testing.T) {
	for i, test := range []struct {
		metaData MetaData
		expected string
	}{
		{MetaData{Title: "Title", Composer: "Composer", Arranger: "Arranger"}, "Title_Composer_Arranger"},
		{MetaData{Title: "", Composer: "", Arranger: ""}, ""},
		{MetaData{Title: "Title", Composer: "", Arranger: ""}, "Title"},
		{MetaData{Title: "", Composer: "Composer", Arranger: ""}, "Composer"},
		{MetaData{Title: "", Composer: "", Arranger: "Arranger"}, "Arranger"},
	} {
		result := test.metaData.String()
		if result != test.expected {
			t.Errorf("Test %d failed. Expected '%s', got '%s'", i, test.expected, result)
		}
	}
}
