package llm

import (
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

func TestIsRetryableHTTPStatus(t *testing.T) {
	if !isRetryableHTTPStatus(429) {
		t.Fatal("429 should be retryable")
	}
	if isRetryableHTTPStatus(404) {
		t.Fatal("404 should not be retryable")
	}
}
