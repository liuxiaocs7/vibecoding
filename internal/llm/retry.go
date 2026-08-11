package llm

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/ymhhh/go-common/logger"
)

// openAIMaxRetries is how many times to retry after the first failure (chat / test).
const openAIMaxRetries = 10

// openAIMaxAttempts = first try + retries.
const openAIMaxAttempts = 1 + openAIMaxRetries

func isRetryableHTTPStatus(status int) bool {
	switch status {
	case 408, 425, 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

func isRetryableNetErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset") ||
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

func logRetry(endpoint, modelName string, attempt, status int, wait time.Duration, cause string) {
	logger.L().WithFields(logger.Fields{
		"url":     endpoint,
		"model":   modelName,
		"attempt": attempt,
		"max":     openAIMaxAttempts,
		"status":  status,
		"wait_ms": wait.Milliseconds(),
		"cause":   truncate(cause, 160),
	}).Warn("llm retrying after failure")
}
