package llm

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/ymhhh/go-common/logger"
)

// RequestTimeout bounds a single chat/spec HTTP handler (SSE included).
const RequestTimeout = 20 * time.Minute

// openAIMaxRetries is how many times to retry after the first failure (429 / 5xx).
const openAIMaxRetries = 10

// openAIMaxAttempts = first try + retries for rate-limit / server errors.
const openAIMaxAttempts = 1 + openAIMaxRetries

// openAITimeoutAttempts is first try + retries for client/gateway timeouts.
const openAITimeoutAttempts = 3

func isRetryableHTTPStatus(status int) bool {
	switch status {
	case 408, 425, 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

func isTimeoutLike(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "client.timeout")
}

func attemptsFor(status int, err error) int {
	if status > 0 && isRetryableHTTPStatus(status) {
		return openAIMaxAttempts
	}
	if isTimeoutLike(err) {
		return openAITimeoutAttempts
	}
	return openAIMaxAttempts
}

func isRetryableNetErr(err error, reqCtx context.Context) bool {
	if err == nil {
		return false
	}
	// Caller cancelled or the handler deadline fired — another attempt in this request is useless.
	if reqCtx != nil && reqCtx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	if isTimeoutLike(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "temporary") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "broken pipe")
}

// retryBackoff returns wait time before the next attempt (attempt is 1-based try that just failed).
func retryBackoff(attempt, status int) time.Duration {
	if status == 429 || (status == 0 && attempt > 0) {
		// Per-minute quota: grow from 5s → 30s.
		d := time.Duration(3+attempt*2) * time.Second
		if status == 429 {
			d = time.Duration(5+attempt*3) * time.Second
		}
		if d > 30*time.Second {
			d = 30 * time.Second
		}
		return d
	}
	// Other transient errors: 1s, 2s, 4s, 8s (cap).
	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 3 {
		shift = 3
	}
	return time.Second << shift
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func logRetry(endpoint, modelName string, attempt, max, status int, wait time.Duration, cause string) {
	if max <= 0 {
		max = openAIMaxAttempts
	}
	logger.L().WithFields(logger.Fields{
		"url":     endpoint,
		"model":   modelName,
		"attempt": attempt,
		"max":     max,
		"status":  status,
		"wait_ms": wait.Milliseconds(),
		"cause":   truncate(cause, 160),
	}).Warn("llm retrying after failure")
}
