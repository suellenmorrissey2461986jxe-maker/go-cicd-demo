package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutes(t *testing.T) {
	originalVersion := version
	version = "test-version"

	t.Cleanup(func() {
		version = originalVersion
	})

	tests := []struct {
		name       string
		path       string
		statusCode int
		body       string
	}{
		{
			name:       "hello endpoint",
			path:       "/",
			statusCode: http.StatusOK,
			body:       "Hello Kubernetes CI/CD\n",
		},
		{
			name:       "health endpoint",
			path:       "/healthz",
			statusCode: http.StatusOK,
			body:       "ok\n",
		},
		{
			name:       "version endpoint",
			path:       "/version",
			statusCode: http.StatusOK,
			body:       "test-version\n",
		},
		{
			name:       "unknown endpoint",
			path:       "/missing",
			statusCode: http.StatusNotFound,
			body:       "404 page not found\n",
		},
	}

	handler := newRouter()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				test.path,
				nil,
			)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != test.statusCode {
				t.Fatalf(
					"expected status %d, got %d",
					test.statusCode,
					recorder.Code,
				)
			}

			if recorder.Body.String() != test.body {
				t.Fatalf(
					"expected body %q, got %q",
					test.body,
					recorder.Body.String(),
				)
			}
		})
	}
}
