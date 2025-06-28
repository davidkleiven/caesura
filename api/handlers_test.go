package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
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

func validMultipartForm() (*bytes.Buffer, string) {
	var multipartBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBuffer)
	multipartWriter.WriteField("composer", "Johan Sebastian Bach")
	multipartWriter.WriteField("title", "Brandenburg Concerto No. 3")
	multipartWriter.CreateFormField("filename.pdf")
	contentWriter, err := multipartWriter.CreateFormFile("document", "filename.pdf")
	if err != nil {
		panic(err)
	}
	pkg.CreateNPagePdf(contentWriter, 10)

	assignments := []pkg.Assignment{
		{Id: "Part1", From: 1, To: 5},
		{Id: "Part2", From: 6, To: 10},
	}
	assignmentWriter, err := multipartWriter.CreateFormField("assignments")
	if err != nil {
		panic(err)
	}
	jsonBytes, err := json.Marshal(assignments)
	if err != nil {
		panic(err)
	}
	assignmentWriter.Write(jsonBytes)
	if err := multipartWriter.Close(); err != nil {
		panic(err)
	}
	return &multipartBuffer, multipartWriter.FormDataContentType()
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

	if files, err := inMemStore.List(""); len(files) != 1 || err != nil {
		t.Errorf("Expected 1 file in store, got %d with error %v", len(files), err)
		return
	}

	// Check content in the store
	content, err := inMemStore.Get("BrandenburgConcertoNo.3_JohanSebastianBach.zip")
	if err != nil {
		t.Errorf("Failed to get file from store: %v", err)
		return
	}

	allContent, err := io.ReadAll(content)
	if err != nil {
		t.Errorf("Failed to read file content: %v", err)
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(allContent), int64(len(allContent)))
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

	var multipartBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBuffer)
	contentWriter, err := multipartWriter.CreateFormFile("document", "filename.txt")
	if err != nil {
		t.Error(err)
		return
	}
	contentWriter.Write([]byte("This is not a PDF file."))
	assignmentWriter, err := multipartWriter.CreateFormField("assignments")
	if err != nil {
		t.Error(err)
		return
	}
	assignmentWriter.Write([]byte("[]"))
	multipartWriter.Close()

	request := httptest.NewRequest("POST", "/submit", &multipartBuffer)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

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
