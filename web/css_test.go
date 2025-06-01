package web

import (
	"net/http/httptest"
	"testing"
)

func TestReceiveCss(t *testing.T) {
	server := CssServer()
	request := httptest.NewRequest("GET", "/css/output.css", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Errorf("Expected status code 200, got %d", response.Code)
	}
}
