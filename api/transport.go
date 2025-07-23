package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/utils"
)

type MockTransport struct {
	UserInfoResponse *http.Response
	TokenResponse    *http.Response
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.String() {
	case googleUserInfo:
		return m.UserInfoResponse, nil
	case googleToken:
		return m.TokenResponse, nil
	default:
		return nil, fmt.Errorf("unknown URL %s", req.URL.String())
	}
}

func NewGoogleUserInfoResponse() *http.Response {
	user := pkg.UserInfo{
		Id:            "217f40fa-c0d7-4d8e-a284-293347868289", // Also present in the pkg.NewDemoStore
		Email:         "testuser@gmail.com",
		VerifiedEmail: true,
		Name:          "Test User",
	}

	bodyBytes := utils.Must(json.Marshal(user))
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}
}

func NewGoogleTokenResponse() *http.Response {
	body := `{
		"access_token": "test-access-token",
		"expires_in": 3600,
		"refresh_token": "test-refresh-token",
		"scope": "https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile",
		"token_type": "Bearer",
		"id_token": "test-id-token"
	}`

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

func NewNotFoundResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewBufferString("Not found")),
	}
}

func NewEmptyResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusNoContent,
	}
}

func NewMockTransport(opts ...func(m *MockTransport)) *MockTransport {
	m := MockTransport{
		UserInfoResponse: NewGoogleUserInfoResponse(),
		TokenResponse:    NewGoogleTokenResponse(),
	}

	for _, opt := range opts {
		opt(&m)
	}
	return &m
}

func WithTokenResponse(resp *http.Response) func(m *MockTransport) {
	return func(m *MockTransport) {
		m.TokenResponse = resp
	}
}

func WithUserInfoResponse(resp *http.Response) func(m *MockTransport) {
	return func(m *MockTransport) {
		m.UserInfoResponse = resp
	}
}
