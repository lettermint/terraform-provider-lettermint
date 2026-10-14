package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lettermint/lettermint-go/v3"
)

const (
	// defaultMaxRetries is how often a rate-limited request is retried when the
	// provider configuration does not set max_retries.
	defaultMaxRetries = 8
	// maxRetriesLimit is the highest max_retries value the provider accepts.
	maxRetriesLimit = 100
	// maxRetryWait is the longest wait before one retry. When the API asks for
	// a longer wait, the provider returns the 429 error instead.
	maxRetryWait = 2 * time.Minute
	// maxBackoff caps the fallback backoff when the API sends no Retry-After.
	maxBackoff = time.Minute
	// maxJitter spreads retries of parallel Terraform operations.
	maxJitter = time.Second
)

var errAttemptTimeout = errors.New("lettermint: attempt timeout")

// rateLimitTransport retries requests that the Lettermint API answers with
// HTTP 429. It waits for the documented Retry-After duration before each
// retry. Every attempt has its own timeout, so waiting does not count against
// the time the API has to answer.
type rateLimitTransport struct {
	base           http.RoundTripper
	maxRetries     int
	attemptTimeout time.Duration
	sleep          func(context.Context, time.Duration) error
	jitter         func() time.Duration
}

func newRateLimitTransport(base http.RoundTripper, maxRetries int) *rateLimitTransport {
	return &rateLimitTransport{
		base:           base,
		maxRetries:     maxRetries,
		attemptTimeout: lettermint.DefaultTimeout,
		sleep:          sleepContext,
		jitter:         func() time.Duration { return rand.N(maxJitter) },
	}
}

// callTimeout is the SDK timeout for one call, including every attempt and
// every wait between attempts.
func (t *rateLimitTransport) callTimeout() time.Duration {
	retries := time.Duration(t.maxRetries)
	return (retries+1)*t.attemptTimeout + retries*(maxRetryWait+maxJitter)
}

func (t *rateLimitTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	ctx := request.Context()
	for attempt := 0; ; attempt++ {
		attemptRequest, err := requestForAttempt(request, attempt)
		if err != nil {
			return nil, err
		}
		response, cancel, err := t.send(attemptRequest)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusTooManyRequests || attempt >= t.maxRetries || !canReplay(request) {
			response.Body = &cancelOnClose{ReadCloser: response.Body, cancel: cancel}
			return response, nil
		}
		wait, ok := retryWait(response.Header.Get("Retry-After"), attempt)
		if !ok {
			response.Body = &cancelOnClose{ReadCloser: response.Body, cancel: cancel}
			return response, nil
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		cancel()

		wait += t.jitter()
		tflog.Warn(ctx, "Lettermint API rate limit reached, retrying", map[string]any{
			"method":      request.Method,
			"path":        request.URL.Path,
			"retry_in":    wait.Round(time.Millisecond).String(),
			"attempt":     attempt + 1,
			"max_retries": t.maxRetries,
		})
		if err := t.sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
}

// send makes one attempt with its own timeout. The caller must call cancel
// after it has read the response body.
func (t *rateLimitTransport) send(request *http.Request) (*http.Response, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeoutCause(request.Context(), t.attemptTimeout, errAttemptTimeout)
	response, err := t.base.RoundTrip(request.WithContext(ctx))
	if err != nil {
		timedOut := request.Context().Err() == nil && errors.Is(context.Cause(ctx), errAttemptTimeout)
		cancel()
		if timedOut {
			return nil, nil, fmt.Errorf("the Lettermint API did not answer within %s", t.attemptTimeout)
		}
		return nil, nil, err
	}
	return response, cancel, nil
}

// requestForAttempt returns the request for the first attempt, and a copy with
// a fresh body for every retry.
func requestForAttempt(request *http.Request, attempt int) (*http.Request, error) {
	if attempt == 0 || request.Body == nil || request.Body == http.NoBody {
		return request, nil
	}
	body, err := request.GetBody()
	if err != nil {
		return nil, err
	}
	retry := request.Clone(request.Context())
	retry.Body = body
	return retry, nil
}

// canReplay reports whether the request body can be sent again.
func canReplay(request *http.Request) bool {
	return request.Body == nil || request.Body == http.NoBody || request.GetBody != nil
}

// retryWait returns the wait before the next attempt. It uses the Retry-After
// header in seconds or as an HTTP date. Without a usable header, it backs off
// exponentially. It returns false when the API asks for a longer wait than
// maxRetryWait.
func retryWait(retryAfter string, attempt int) (time.Duration, bool) {
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
			wait := time.Duration(seconds) * time.Second
			return wait, wait <= maxRetryWait
		}
		if date, err := http.ParseTime(retryAfter); err == nil {
			wait := max(time.Until(date), 0)
			return wait, wait <= maxRetryWait
		}
	}
	return min(time.Second<<min(attempt, 6), maxBackoff), true
}

func sleepContext(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return nil
	}
}

// cancelOnClose ends the attempt context when the SDK closes the response body.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
