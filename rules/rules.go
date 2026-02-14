package rules

import (
	"fmt"
	"time"
)

// Edge represents a single directed call between two services.
type Edge struct {
	Source            string
	Target            string
	Timeout           time.Duration
	MaxRetries        int
	Idempotent        bool
	HasCircuitBreaker bool
	HasBackoff        bool
	Jitter            bool
}

// CallGraph is the minimal interface that rules need to inspect a service
// topology. Implementations live outside this package; tests use a mock.
type CallGraph interface {
	AllEdges() []Edge
	OutEdges(node string) []Edge
	Paths() [][]Edge
}

// Violation describes a single anti-pattern finding.
type Violation struct {
	Rule       string
	Severity   string
	Path       []string
	Message    string
	SourceHint string
}

// Rule is the interface every anti-pattern detector must implement.
type Rule interface {
	Check(graph CallGraph) []Violation
}

// pathNodes extracts the ordered node names from a slice of edges.
func pathNodes(edges []Edge) []string {
	if len(edges) == 0 {
		return nil
	}
	nodes := make([]string, 0, len(edges)+1)
	nodes = append(nodes, edges[0].Source)
	for _, e := range edges {
		nodes = append(nodes, e.Target)
	}
	return nodes
}

// ---------------------------------------------------------------------------
// Rule 1: TimeoutInversionRule
// ---------------------------------------------------------------------------

// TimeoutInversionRule detects adjacent edge pairs where the downstream
// edge's timeout exceeds the upstream edge's timeout.
type TimeoutInversionRule struct{}

func (r *TimeoutInversionRule) Check(graph CallGraph) []Violation {
	var violations []Violation
	for _, e := range graph.AllEdges() {
		for _, d := range graph.OutEdges(e.Target) {
			if e.Timeout > 0 && d.Timeout > e.Timeout {
				violations = append(violations, Violation{
					Rule:     "timeout-inversion",
					Severity: "error",
					Path:     []string{e.Source, e.Target, d.Target},
					Message: fmt.Sprintf(
						"%s->%s timeout %v but %s->%s timeout %v (downstream > upstream)",
						e.Source, e.Target, e.Timeout, e.Target, d.Target, d.Timeout),
					SourceHint: fmt.Sprintf("edge %s->%s", e.Source, e.Target),
				})
			}
		}
	}
	return violations
}

// ---------------------------------------------------------------------------
// Rule 2: RetryAmplificationRule
// ---------------------------------------------------------------------------

// RetryAmplificationRule checks that the multiplicative retry factor along
// any root-to-leaf path does not exceed configurable thresholds.
type RetryAmplificationRule struct {
	ErrorThreshold   int // product > this → error   (default 10)
	WarningThreshold int // product > this → warning  (default 5)
}

func (r *RetryAmplificationRule) Check(graph CallGraph) []Violation {
	errT := r.ErrorThreshold
	if errT == 0 {
		errT = 10
	}
	warnT := r.WarningThreshold
	if warnT == 0 {
		warnT = 5
	}

	var violations []Violation
	for _, path := range graph.Paths() {
		product := 1
		for _, e := range path {
			product *= (1 + e.MaxRetries)
		}
		if product > errT {
			violations = append(violations, Violation{
				Rule:     "retry-amplification",
				Severity: "error",
				Path:     pathNodes(path),
				Message:  fmt.Sprintf("retry amplification factor %d exceeds error threshold %d", product, errT),
			})
		} else if product > warnT {
			violations = append(violations, Violation{
				Rule:     "retry-amplification",
				Severity: "warning",
				Path:     pathNodes(path),
				Message:  fmt.Sprintf("retry amplification factor %d exceeds warning threshold %d", product, warnT),
			})
		}
	}
	return violations
}

// ---------------------------------------------------------------------------
// Rule 3: NonIdempotentRetryRule
// ---------------------------------------------------------------------------

// NonIdempotentRetryRule flags edges where Idempotent==false yet
// MaxRetries > 0.
type NonIdempotentRetryRule struct{}

func (r *NonIdempotentRetryRule) Check(graph CallGraph) []Violation {
	var violations []Violation
	for _, e := range graph.AllEdges() {
		if !e.Idempotent && e.MaxRetries > 0 {
			violations = append(violations, Violation{
				Rule:     "non-idempotent-retry",
				Severity: "error",
				Path:     []string{e.Source, e.Target},
				Message: fmt.Sprintf(
					"%s->%s retries %d times but is not idempotent",
					e.Source, e.Target, e.MaxRetries),
				SourceHint: fmt.Sprintf("edge %s->%s", e.Source, e.Target),
			})
		}
	}
	return violations
}

// ---------------------------------------------------------------------------
// Rule 4: RetryWithoutCircuitBreakerRule
// ---------------------------------------------------------------------------

// RetryWithoutCircuitBreakerRule flags edges that have retries configured
// but no circuit breaker.
type RetryWithoutCircuitBreakerRule struct{}

func (r *RetryWithoutCircuitBreakerRule) Check(graph CallGraph) []Violation {
	var violations []Violation
	for _, e := range graph.AllEdges() {
		if e.MaxRetries > 0 && !e.HasCircuitBreaker {
			violations = append(violations, Violation{
				Rule:     "retry-without-cb",
				Severity: "warning",
				Path:     []string{e.Source, e.Target},
				Message: fmt.Sprintf(
					"%s->%s has %d retries but no circuit breaker",
					e.Source, e.Target, e.MaxRetries),
				SourceHint: fmt.Sprintf("edge %s->%s", e.Source, e.Target),
			})
		}
	}
	return violations
}

// ---------------------------------------------------------------------------
// Rule 5: BackoffWithoutJitterRule
// ---------------------------------------------------------------------------

// BackoffWithoutJitterRule flags edges that have a backoff strategy but
// no jitter, risking thundering-herd effects.
type BackoffWithoutJitterRule struct{}

func (r *BackoffWithoutJitterRule) Check(graph CallGraph) []Violation {
	var violations []Violation
	for _, e := range graph.AllEdges() {
		if e.HasBackoff && !e.Jitter {
			violations = append(violations, Violation{
				Rule:     "backoff-no-jitter",
				Severity: "warning",
				Path:     []string{e.Source, e.Target},
				Message: fmt.Sprintf(
					"%s->%s has backoff but no jitter (thundering herd risk)",
					e.Source, e.Target),
				SourceHint: fmt.Sprintf("edge %s->%s", e.Source, e.Target),
			})
		}
	}
	return violations
}

// ---------------------------------------------------------------------------
// Rule 6: EndToEndTimeoutExceedRule
// ---------------------------------------------------------------------------

// EndToEndTimeoutExceedRule checks that the worst-case end-to-end latency
// of every path does not exceed a configurable entry timeout.
// Worst-case latency per edge = Timeout × (1 + MaxRetries).
type EndToEndTimeoutExceedRule struct {
	EntryTimeout time.Duration
}

func (r *EndToEndTimeoutExceedRule) Check(graph CallGraph) []Violation {
	if r.EntryTimeout == 0 {
		return nil
	}
	var violations []Violation
	for _, path := range graph.Paths() {
		var worstCase time.Duration
		for _, e := range path {
			worstCase += e.Timeout * time.Duration(1+e.MaxRetries)
		}
		if worstCase > r.EntryTimeout {
			violations = append(violations, Violation{
				Rule:     "e2e-timeout-exceed",
				Severity: "error",
				Path:     pathNodes(path),
				Message: fmt.Sprintf(
					"worst-case latency %v exceeds entry timeout %v",
					worstCase, r.EntryTimeout),
			})
		}
	}
	return violations
}
