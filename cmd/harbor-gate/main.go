// Harbor scan gate. Exit codes: 0 PASS, 2 ERROR, 3 BLOCK.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

const reportMIME = "application/vnd.security.vulnerability.report; version=1.1"
const artifactBase = "http://100.113.248.106:30002/api/v2.0/projects/go-cicd-demo/repositories/go-cicd-demo/artifacts/"

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type Summary struct {
	Total  *int           `json:"total"`
	Counts map[string]int `json:"summary"`
}

type Report struct {
	ID       string    `json:"report_id"`
	Status   string    `json:"scan_status"`
	Complete int       `json:"complete_percent"`
	End      time.Time `json:"end_time"`
	Summary  *Summary  `json:"summary"`
}

type Artifact struct {
	Digest   string            `json:"digest"`
	Overview map[string]Report `json:"scan_overview"`
}

func evaluate(artifact Artifact, expected string) (int, string) {
	if !digestPattern.MatchString(expected) || artifact.Digest != expected {
		return 2, "artifact digest does not match the expected digest"
	}
	report, exists := artifact.Overview[reportMIME]
	if !exists || report.Status != "Success" || report.Complete != 100 || report.End.IsZero() {
		return 2, "scan has not completed successfully"
	}
	if report.Summary == nil || report.Summary.Total == nil || report.Summary.Counts == nil || *report.Summary.Total < 0 {
		return 2, "vulnerability summary is missing or invalid"
	}
	total := 0
	for severity, count := range report.Summary.Counts {
		if count < 0 {
			return 2, "negative vulnerability count"
		}
		switch severity {
		case "Critical", "High", "Medium", "Low", "Negligible":
		case "Unknown", "None":
			if count != 0 {
				return 2, "unclassified or inconsistent vulnerability data"
			}
		default:
			return 2, "unrecognized severity category"
		}
		// Also prevents integer overflow while adding untrusted counts.
		if count > *report.Summary.Total-total {
			return 2, "vulnerability counts are inconsistent"
		}
		total += count
	}
	if total != *report.Summary.Total {
		return 2, "vulnerability counts are inconsistent"
	}
	critical, high := report.Summary.Counts["Critical"], report.Summary.Counts["High"]
	detail := fmt.Sprintf("Total=%d Critical=%d High=%d", total, critical, high)
	if critical > 0 || high > 0 {
		return 3, "High or Critical vulnerabilities found; " + detail
	}
	return 0, "no High or Critical vulnerabilities; " + detail
}

type harborClient struct {
	client                   *http.Client
	base, username, password string
	interval                 time.Duration
}

func (h harborClient) request(ctx context.Context, method, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create Harbor request")
	}
	req.SetBasicAuth(h.username, h.password)
	req.Header.Set("Accept", "application/json")
	response, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Harbor request failed or timed out")
	}
	return response, nil
}

func (h harborClient) get(ctx context.Context, reference, expected string) (Artifact, error) {
	var artifact Artifact
	response, err := h.request(ctx, http.MethodGet, h.base+url.PathEscape(reference)+"?with_scan_overview=true")
	if err != nil {
		return artifact, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return artifact, fmt.Errorf("Harbor GET returned HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, (2<<20)+1))
	if err := decoder.Decode(&artifact); err != nil {
		return artifact, fmt.Errorf("invalid artifact JSON")
	}
	var extra interface{}
	if decoder.Decode(&extra) != io.EOF {
		return artifact, fmt.Errorf("trailing or oversized artifact JSON")
	}
	if artifact.Digest != expected {
		return artifact, fmt.Errorf("artifact digest mismatch")
	}
	return artifact, nil
}

func pause(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("scan wait timed out or was canceled")
	case <-timer.C:
		return nil
	}
}

// Do not trust a pre-existing successful report after POSTing a new scan.
// Missing IDs or an unchanged report fail closed (eventually timeout).
func fresh(report, previous Report) bool {
	return report.ID != "" && report.ID != previous.ID && !report.End.IsZero() &&
		(previous.End.IsZero() || report.End.After(previous.End))
}

func (h harborClient) scan(ctx context.Context, digest string, out io.Writer) (Artifact, error) {
	before, err := h.get(ctx, digest, digest)
	if err != nil {
		return Artifact{}, err
	}
	previous := before.Overview[reportMIME]
	response, err := h.request(ctx, http.MethodPost, h.base+url.PathEscape(digest)+"/scan")
	if err != nil {
		return Artifact{}, err
	}
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return Artifact{}, fmt.Errorf("scan POST returned HTTP %d; check scan permission or an existing scan; deployment blocked", response.StatusCode)
	}
	fmt.Fprintln(out, "Scan accepted for digest:", digest)
	for {
		if err := pause(ctx, h.interval); err != nil {
			return Artifact{}, err
		}
		artifact, err := h.get(ctx, digest, digest)
		if err != nil {
			return Artifact{}, err
		}
		report, exists := artifact.Overview[reportMIME]
		if !exists {
			fmt.Fprintln(out, "Waiting for scan overview")
			continue
		}
		fmt.Fprintf(out, "Scan status=%s progress=%d%%\n", report.Status, report.Complete)
		// The API may briefly return the previous completed or failed report.
		if report.ID == previous.ID {
			continue
		}
		switch report.Status {
		case "Success":
			if fresh(report, previous) {
				return artifact, nil
			}
		case "Error", "Stopped", "Failed":
			return Artifact{}, fmt.Errorf("scan ended with status %s", report.Status)
		case "", "Pending", "Scheduled", "Running":
		default:
			return Artifact{}, fmt.Errorf("unrecognized scan status %q", report.Status)
		}
	}
}

func metadataDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot read BuildKit metadata")
	}
	defer file.Close()
	var metadata struct {
		Digest string `json:"containerimage.digest"`
	}
	decoder := json.NewDecoder(io.LimitReader(file, 2<<20))
	if decoder.Decode(&metadata) != nil || !digestPattern.MatchString(metadata.Digest) {
		return "", fmt.Errorf("missing or invalid containerimage.digest in BuildKit metadata")
	}
	var extra interface{}
	if decoder.Decode(&extra) != io.EOF {
		return "", fmt.Errorf("invalid metadata JSON trailer")
	}
	return metadata.Digest, nil
}

func run(args []string, out io.Writer) int {
	errorResult := func(err error) int { fmt.Fprintln(out, "ERROR:", err); return 2 }
	if len(args) != 3 || (args[0] != "check" && args[0] != "scan") {
		return errorResult(fmt.Errorf("usage: gate check TAG EXPECTED_DIGEST | gate scan METADATA_FILE DIGEST_OUTPUT_FILE"))
	}
	if args[0] == "scan" {
		if args[1] == args[2] {
			return errorResult(fmt.Errorf("input and output paths must differ"))
		}
		if err := os.Remove(args[2]); err != nil && !os.IsNotExist(err) {
			return errorResult(err)
		}
	}
	username, password := os.Getenv("SCAN_USER"), os.Getenv("SCAN_SECRET")
	if username == "" || password == "" {
		return errorResult(fmt.Errorf("missing Harbor credentials"))
	}
	h := harborClient{
		client: &http.Client{
			Timeout:       20 * time.Second,
			Transport:     &http.Transport{Proxy: nil},
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		},
		base: artifactBase, username: username, password: password, interval: 5 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	digest := args[2]
	var artifact Artifact
	var err error
	if args[0] == "scan" {
		digest, err = metadataDigest(args[1])
		if err == nil {
			artifact, err = h.scan(ctx, digest, out)
		}
	} else {
		if !digestPattern.MatchString(digest) {
			return errorResult(fmt.Errorf("invalid expected digest"))
		}
		artifact, err = h.get(ctx, args[1], digest)
	}
	if err != nil {
		return errorResult(err)
	}
	code, message := evaluate(artifact, digest)
	fmt.Fprintln(out, "Digest:", digest)
	fmt.Fprintln(out, "Scan completed:", artifact.Overview[reportMIME].End.Format(time.RFC3339))
	if code == 0 && args[0] == "scan" {
		if err := os.WriteFile(args[2], []byte(digest+"\n"), 0644); err != nil {
			return errorResult(err)
		}
	}
	labels := map[int]string{0: "PASS", 2: "ERROR", 3: "BLOCK"}
	fmt.Fprintln(out, labels[code]+":", message)
	return code
}

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }
