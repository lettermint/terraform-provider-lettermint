package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/lettermint/lettermint-go/v3"
)

type rateLimitRecorder struct {
	statuses []int
	headers  []string
	bodies   []string
	waits    []time.Duration
}

func (r *rateLimitRecorder) transport(maxRetries int) *rateLimitTransport {
	transport := newRateLimitTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := ""
		if request.Body != nil {
			data, err := io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
			body = string(data)
		}
		attempt := len(r.bodies)
		r.bodies = append(r.bodies, body)
		status := http.StatusOK
		if attempt < len(r.statuses) {
			status = r.statuses[attempt]
		}
		header := http.Header{"Content-Type": []string{"application/json"}}
		if attempt < len(r.headers) && r.headers[attempt] != "" {
			header.Set("Retry-After", r.headers[attempt])
		}
		return &http.Response{
			StatusCode: status,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(`{"message":"Too many requests."}`)),
			Request:    request,
		}, nil
	}), maxRetries)
	transport.sleep = func(_ context.Context, wait time.Duration) error {
		r.waits = append(r.waits, wait)
		return nil
	}
	transport.jitter = func() time.Duration { return 0 }
	return transport
}

func doRateLimited(t *testing.T, transport http.RoundTripper, method string, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, "https://api.example.test/v1/projects", reader)
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		t.Fatalf("request error = %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	return response
}

func TestRateLimitTransportWaitsForRetryAfter(t *testing.T) {
	t.Parallel()

	recorder := &rateLimitRecorder{
		statuses: []int{http.StatusTooManyRequests, http.StatusTooManyRequests},
		headers:  []string{"2", "7"},
	}
	response := doRateLimited(t, recorder.transport(8), http.MethodGet, "")

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if len(recorder.bodies) != 3 {
		t.Fatalf("attempts = %d, want 3", len(recorder.bodies))
	}
	if len(recorder.waits) != 2 || recorder.waits[0] != 2*time.Second || recorder.waits[1] != 7*time.Second {
		t.Fatalf("waits = %v, want [2s 7s]", recorder.waits)
	}
}

func TestRateLimitTransportReplaysRequestBody(t *testing.T) {
	t.Parallel()

	recorder := &rateLimitRecorder{statuses: []int{http.StatusTooManyRequests}, headers: []string{"1"}}
	response := doRateLimited(t, recorder.transport(8), http.MethodPost, `{"name":"Example"}`)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if len(recorder.bodies) != 2 || recorder.bodies[0] != `{"name":"Example"}` || recorder.bodies[1] != `{"name":"Example"}` {
		t.Fatalf("bodies = %q, want the same body twice", recorder.bodies)
	}
}

func TestRateLimitTransportReadsRetryAfterDate(t *testing.T) {
	t.Parallel()

	date := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	recorder := &rateLimitRecorder{statuses: []int{http.StatusTooManyRequests}, headers: []string{date}}
	doRateLimited(t, recorder.transport(8), http.MethodGet, "")

	if len(recorder.waits) != 1 || recorder.waits[0] <= 25*time.Second || recorder.waits[0] > 30*time.Second {
		t.Fatalf("waits = %v, want about 30s", recorder.waits)
	}
}

func TestRateLimitTransportBacksOffWithoutRetryAfter(t *testing.T) {
	t.Parallel()

	recorder := &rateLimitRecorder{statuses: []int{http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests}}
	doRateLimited(t, recorder.transport(8), http.MethodGet, "")

	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if len(recorder.waits) != len(want) {
		t.Fatalf("waits = %v, want %v", recorder.waits, want)
	}
	for index := range want {
		if recorder.waits[index] != want[index] {
			t.Fatalf("waits = %v, want %v", recorder.waits, want)
		}
	}
}

func TestRateLimitTransportDoesNotWaitLongerThanTheMaximum(t *testing.T) {
	t.Parallel()

	recorder := &rateLimitRecorder{statuses: []int{http.StatusTooManyRequests}, headers: []string{"3600"}}
	response := doRateLimited(t, recorder.transport(8), http.MethodGet, "")

	if response.StatusCode != http.StatusTooManyRequests || len(recorder.bodies) != 1 || len(recorder.waits) != 0 {
		t.Fatalf("status = %d, attempts = %d, waits = %v", response.StatusCode, len(recorder.bodies), recorder.waits)
	}
}

func TestRateLimitTransportWithoutRetries(t *testing.T) {
	t.Parallel()

	recorder := &rateLimitRecorder{statuses: []int{http.StatusTooManyRequests}, headers: []string{"1"}}
	response := doRateLimited(t, recorder.transport(0), http.MethodGet, "")

	if response.StatusCode != http.StatusTooManyRequests || len(recorder.bodies) != 1 {
		t.Fatalf("status = %d, attempts = %d", response.StatusCode, len(recorder.bodies))
	}
}

func TestRateLimitTransportStopsWhenContextIsCanceled(t *testing.T) {
	t.Parallel()

	recorder := &rateLimitRecorder{statuses: []int{http.StatusTooManyRequests, http.StatusTooManyRequests}, headers: []string{"5", "5"}}
	transport := recorder.transport(8)
	ctx, cancel := context.WithCancel(context.Background())
	transport.sleep = func(ctx context.Context, wait time.Duration) error {
		cancel()
		return sleepContext(ctx, wait)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.test/v1/projects", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := transport.RoundTrip(request); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if len(recorder.bodies) != 1 {
		t.Fatalf("attempts = %d, want 1", len(recorder.bodies))
	}
}

func TestRateLimitTransportTimesOutEachAttempt(t *testing.T) {
	t.Parallel()

	transport := newRateLimitTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	}), 8)
	transport.attemptTimeout = 10 * time.Millisecond
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.example.test/v1/projects", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = transport.RoundTrip(request)
	if err == nil || !strings.Contains(err.Error(), "did not answer within 10ms") {
		t.Fatalf("error = %v, want an attempt timeout", err)
	}
}

func TestRateLimitTransportReturnsRateLimitErrorAfterRetries(t *testing.T) {
	t.Parallel()

	recorder := &rateLimitRecorder{
		statuses: []int{http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests},
		headers:  []string{"1", "1", "1"},
	}
	transport := recorder.transport(2)
	client, err := lettermint.New(
		lettermint.WithTeamToken("team-secret"),
		lettermint.WithBaseURL("https://api.example.test/v1"),
		lettermint.WithTimeout(transport.callTimeout()),
		lettermint.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Team.Retrieve(context.Background(), nil)
	var rateLimit *lettermint.RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("error = %v, want *lettermint.RateLimitError", err)
	}
	if len(recorder.bodies) != 3 {
		t.Fatalf("attempts = %d, want 3", len(recorder.bodies))
	}

	var diagnostics diag.Diagnostics
	appendClientDiagnostic(&diagnostics, "Cannot read team", err)
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Detail(), "-parallelism") {
		t.Fatalf("diagnostics = %v, want a rate limit hint", diagnostics)
	}
}
