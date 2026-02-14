package main

import (
	"testing"
	"time"
)

func hasRule(findings []Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func edge(src, tgt string, timeout time.Duration, retries int, cb bool, method string, jitter bool) CallEdge {
	return CallEdge{Source: src, Target: tgt, Timeout: timeout, Retries: retries,
		CircuitBreaker: cb, Method: method, BackoffJitter: jitter}
}

func TestTimeoutInversion(t *testing.T) {
	g := NewGraph([]CallEdge{
		edge("A", "B", 3*time.Second, 0, true, "GET", true),
		edge("B", "C", 5*time.Second, 0, true, "GET", true),
	})
	findings := g.Analyze()
	if !hasRule(findings, "timeout-inversion") {
		t.Fatal("expected timeout-inversion: B->C 5s > A->B 3s")
	}
}

func TestRetryAmplification(t *testing.T) {
	g := NewGraph([]CallEdge{
		edge("A", "B", 5*time.Second, 3, true, "GET", true),
		edge("B", "C", 3*time.Second, 3, true, "GET", true),
	})
	findings := g.Analyze()
	if !hasRule(findings, "retry-amplification") {
		t.Fatal("expected retry-amplification: (1+3)*(1+3)=16 > 10")
	}
}

func TestRetryWithoutCircuitBreaker(t *testing.T) {
	g := NewGraph([]CallEdge{
		edge("A", "B", 3*time.Second, 2, false, "GET", true),
	})
	if !hasRule(g.Analyze(), "retry-without-cb") {
		t.Fatal("expected retry-without-cb")
	}
}

func TestNonIdempotentRetry(t *testing.T) {
	g := NewGraph([]CallEdge{
		edge("A", "B", 3*time.Second, 2, true, "POST", true),
	})
	if !hasRule(g.Analyze(), "non-idempotent-retry") {
		t.Fatal("expected non-idempotent-retry for POST")
	}
}

func TestBackoffNoJitter(t *testing.T) {
	g := NewGraph([]CallEdge{
		edge("A", "B", 3*time.Second, 2, true, "GET", false),
	})
	if !hasRule(g.Analyze(), "backoff-no-jitter") {
		t.Fatal("expected backoff-no-jitter")
	}
}

func TestCleanTopology(t *testing.T) {
	g := NewGraph([]CallEdge{
		edge("gateway", "api", 5*time.Second, 1, true, "GET", true),
		edge("api", "db", 3*time.Second, 1, true, "GET", true),
	})
	findings := g.Analyze()
	if len(findings) != 0 {
		for _, f := range findings {
			t.Logf("unexpected: [%s] %s", f.Rule, f.Message)
		}
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}
