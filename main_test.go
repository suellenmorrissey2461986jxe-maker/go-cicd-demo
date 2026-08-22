package main

import (
"net/http"
"net/http/httptest"
"testing"
)

func TestHelloHandler(t *testing.T) {
req := httptest.NewRequest(http.MethodGet, "/", nil)
recorder := httptest.NewRecorder()

helloHandler(recorder, req)

if recorder.Code != http.StatusOK {
t.Fatalf("expected status 200, got %d", recorder.Code)
}

expected := "Hello Kubernetes CI/CD\n"
if recorder.Body.String() != expected {
t.Fatalf("expected %q, got %q", expected, recorder.Body.String())
}
}
