package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func cleanArtifact() Artifact {
	zero := 0
	return Artifact{Digest: testDigest, Overview: map[string]Report{reportMIME: {
		ID: "new", Status: "Success", Complete: 100,
		End:     time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC),
		Summary: &Summary{Total: &zero, Counts: map[string]int{}},
	}}}
}

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Report)
		want   int
	}{
		{"clean", func(r *Report) {}, 0},
		{"medium allowed", func(r *Report) { *r.Summary.Total = 2; r.Summary.Counts["Medium"] = 2 }, 0},
		{"critical blocked", func(r *Report) { *r.Summary.Total = 1; r.Summary.Counts["Critical"] = 1 }, 3},
		{"high blocked", func(r *Report) { *r.Summary.Total = 21; r.Summary.Counts["High"] = 21 }, 3},
		{"not successful", func(r *Report) { r.Status = "Running" }, 2},
		{"incomplete", func(r *Report) { r.Complete = 99 }, 2},
		{"missing end", func(r *Report) { r.End = time.Time{} }, 2},
		{"missing summary", func(r *Report) { r.Summary = nil }, 2},
		{"missing total", func(r *Report) { r.Summary.Total = nil }, 2},
		{"missing counts", func(r *Report) { r.Summary.Counts = nil }, 2},
		{"negative total", func(r *Report) { *r.Summary.Total = -1 }, 2},
		{"negative count", func(r *Report) { r.Summary.Counts["Low"] = -1 }, 2},
		{"inconsistent total", func(r *Report) { *r.Summary.Total = 2 }, 2},
		{"count exceeds total", func(r *Report) { r.Summary.Counts["Low"] = 1 }, 2},
		{"unknown nonzero", func(r *Report) { *r.Summary.Total = 1; r.Summary.Counts["Unknown"] = 1 }, 2},
		{"unknown category", func(r *Report) { r.Summary.Counts["FutureSeverity"] = 0 }, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := cleanArtifact()
			r := a.Overview[reportMIME]
			tc.mutate(&r)
			a.Overview[reportMIME] = r
			if got, msg := evaluate(a, testDigest); got != tc.want {
				t.Fatalf("got %d (%s), want %d", got, msg, tc.want)
			}
		})
	}
	a := cleanArtifact()
	a.Digest = "sha256:wrong"
	if got, _ := evaluate(a, testDigest); got != 2 {
		t.Fatal("digest mismatch passed")
	}
	a = cleanArtifact()
	a.Overview = nil
	if got, _ := evaluate(a, testDigest); got != 2 {
		t.Fatal("missing report passed")
	}
}

func TestScanWaitsForFreshReport(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		postStatus   int
		wantError    bool
		wantCode     int
	}{
		{"pass", "Success", 202, false, 0},
		{"block", "SuccessHigh", 202, false, 3},
		{"scan error", "Error", 202, true, 0},
		{"forbidden", "Success", 403, true, 0},
		{"conflict", "Success", 409, true, 0},
		{"server error", "Success", 502, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gets, posts := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, password, ok := r.BasicAuth()
				if !ok || user != "robot" || password != "secret" {
					t.Error("missing credentials")
				}
				if !strings.Contains(r.URL.Path, testDigest) {
					t.Error("request was not by digest")
				}
				if r.Method == http.MethodPost {
					posts++
					w.WriteHeader(tc.postStatus)
					return
				}
				gets++
				a := cleanArtifact()
				report := a.Overview[reportMIME]
				if gets <= 2 {
					report.ID = "old"
					report.End = report.End.Add(-time.Hour)
				} else {
					report.Status = tc.status
					if tc.status == "SuccessHigh" {
						report.Status = "Success"
						*report.Summary.Total = 1
						report.Summary.Counts["High"] = 1
					}
				}
				a.Overview[reportMIME] = report
				json.NewEncoder(w).Encode(a)
			}))
			defer server.Close()
			h := harborClient{client: server.Client(), base: server.URL + "/", username: "robot", password: "secret", interval: time.Millisecond}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			a, err := h.scan(ctx, testDigest, io.Discard)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if posts != 1 {
				t.Fatalf("expected one POST, got %d", posts)
			}
			if !tc.wantError {
				if gets < 3 {
					t.Fatal("accepted the old report")
				}
				if code, _ := evaluate(a, testDigest); code != tc.wantCode {
					t.Fatalf("code=%d", code)
				}
			}
		})
	}
}

func TestStaleReportTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(202)
			return
		}
		json.NewEncoder(w).Encode(cleanArtifact())
	}))
	defer server.Close()
	h := harborClient{client: server.Client(), base: server.URL + "/", interval: time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := h.scan(ctx, testDigest, io.Discard); err == nil {
		t.Fatal("stale report passed")
	}
}

func TestMetadataDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.json")
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"containerimage.digest":"` + testDigest + `"}`, true},
		{`{"containerimage.config.digest":"` + testDigest + `"}`, false},
		{`{"containerimage.digest":"sha256:bad"}`, false},
		{`{`, false},
		{`{"containerimage.digest":"` + testDigest + `"} {}`, false},
	} {
		if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := metadataDigest(path)
		if (err == nil) != tc.valid {
			t.Fatalf("unexpected result for %s: %v", tc.body, err)
		}
	}
}

func TestRunRemovesStaleOutputOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digest")
	if err := os.WriteFile(path, []byte(testDigest), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SCAN_USER", "")
	t.Setenv("SCAN_SECRET", "")
	if code := run([]string{"scan", "missing-metadata", path}, io.Discard); code != 2 {
		t.Fatalf("code=%d", code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stale digest output remains")
	}
}
