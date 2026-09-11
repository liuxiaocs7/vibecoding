package llm

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRetryBackoff429GrowsAndCaps(t *testing.T) {
	prev := time.Duration(0)
	for attempt := 1; attempt <= openAIMaxRetries; attempt++ {
		d := retryBackoff(attempt, 429)
		if d < prev {
			t.Fatalf("backoff should not shrink: attempt=%d got=%v prev=%v", attempt, d, prev)
		}
		if d > 30*time.Second {
			t.Fatalf("backoff cap 30s exceeded: %v", d)
		}
		prev = d
	}
	if retryBackoff(1, 429) < 5*time.Second {
		t.Fatalf("429 first wait too short: %v", retryBackoff(1, 429))
	}
}

func TestIsRetryableNetErrClientTimeoutWhileCtxAlive(t *testing.T) {
	ctx := context.Background()
	err := fmt.Errorf("OpenAPI request failed [200] url=http://x: context deadline exceeded (Client.Timeout or context cancellation while reading body)")
	if !isRetryableNetErr(err, ctx) {
		t.Fatal("client timeout should be retryable while request context is alive")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if isRetryableNetErr(err, canceled) {
		t.Fatal("should not retry after caller context is done")
	}
	if isRetryableNetErr(context.Canceled, ctx) {
		t.Fatal("explicit cancel should not be retryable")
	}
}

func TestAttemptsForTimeoutVsRateLimit(t *testing.T) {
	if attemptsFor(429, nil) != openAIMaxAttempts {
		t.Fatalf("429 attempts=%d", attemptsFor(429, nil))
	}
	if attemptsFor(0, fmt.Errorf("context deadline exceeded (Client.Timeout)")) != openAITimeoutAttempts {
		t.Fatalf("timeout attempts=%d", attemptsFor(0, fmt.Errorf("timeout")))
	}
}
