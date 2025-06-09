package api

import (
	"net/http/httptest"
	"strings"
	"testing"
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

	expectedResponse := "flute\n<input type=\"text\" placeholder=\"Enter part number\" id=\"part-number\"/>"
	if recorder.Body.String() != expectedResponse {
		t.Errorf("Expected response body to be '%s', got '%s'", expectedResponse, recorder.Body.String())
	}
}
