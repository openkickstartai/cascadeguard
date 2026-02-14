package rules

import (
	"sort"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock CallGraph implementation
// ---------------------------------------------------------------------------

type mockGraph struct {
	edges []Edge
	adj   map[string][]Edge
}

func newMockGraph(edges ...Edge) *mockGraph {
	adj := make(map[string][]Edge)
	for _, e := range edges {
		adj[e.Source] = append(adj[e.Source], e)
	}
	return &mockGraph{edges: edges, adj: adj}
}

func (g *mockGraph) AllEdges() []Edge            { return g.edges }
func (g *mockGraph) OutEdges(node string) []Edge  { return g.adj[node] }

// Paths enumerates all root-to-leaf paths via DFS.
func (g *mockGraph) Paths() [][]Edge {
	targets := make(map[string]bool)
	sources := make(map[string]bool)
	for _, e := range g.edges {
		targets[e.Target] = true
		sources[e.Source] = true
	}
	var roots []string
	for s := range sources {
		if !targets[s] {
			roots = append(roots, s)
		}
	}
	if len(roots) == 0 {
		for s := range sources {
			roots = append(roots, s)
		}
	}
	// Sort for deterministic output across runs.
	sort.Strings(roots)

	var paths [][]Edge
	for _, root := range roots {
		visited := map[string]bool{root: true}
		g.dfs(root, nil, &paths, visited)
	}
	return paths
}

func (g *mockGraph) dfs(node string, current []Edge, paths *[][]Edge, visited map[string]bool) {
	out := g.adj[node]
	if len(out) == 0 {
		if len(current) > 0 {
			cp := make([]Edge, len(current))
			copy(cp, current)
			*paths = append(*paths, cp)
		}
		return
	}
	for _, e := range out {
		if visited[e.Target] {
			continue
		}
		visited[e.Target] = true
		next := make([]Edge, len(current)+1)
		copy(next, current)
		next[len(current)] = e
		g.dfs(e.Target, next, paths, visited)
		delete(visited, e.Target)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func hasRule(vs []Violation, rule string) bool {
	for _, v := range vs {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

func hasSeverity(vs []Violation, rule, severity string) bool {
	for _, v := range vs {
		if v.Rule == rule && v.Severity == severity {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Rule 1: TimeoutInversionRule
// ---------------------------------------------------------------------------

func TestTimeoutInversionRule(t *testing.T) {
	tests := []struct {
		name   string
		edges  []Edge
		want   bool
	}{
		{
			name: "downstream timeout > upstream — triggers",
			edges: []Edge{
				{Source: "A", Target: "B", Timeout: 3 * time.Second},
				{Source: "B", Target: "C", Timeout: 5 * time.Second},
			},
			want: true,
		},
		{
			name: "downstream timeout < upstream — clean",
			edges: []Edge{
				{Source: "A", Target: "B", Timeout: 5 * time.Second},
				{Source: "B", Target: "C", Timeout: 3 * time.Second},
			},
			want: false,
		},
		{
			name: "equal timeouts — clean",
			edges: []Edge{
				{Source: "A", Target: "B", Timeout: 3 * time.Second},
				{Source: "B", Target: "C", Timeout: 3 * time.Second},
			},
			want: false,
		},
		{
			name: "upstream timeout zero — clean",
			edges: []Edge{
				{Source: "A", Target: "B", Timeout: 0},
				{Source: "B", Target: "C", Timeout: 5 * time.Second},
			},
			want: false,
		},
	}

	rule := &TimeoutInversionRule{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newMockGraph(tc.edges...)
			vs := rule.Check(g)
			got := hasRule(vs, "timeout-inversion")
			if got != tc.want {
				t.Errorf("expected violation=%v, got %v; violations=%+v", tc.want, got, vs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Rule 2: RetryAmplificationRule
// ---------------------------------------------------------------------------

func TestRetryAmplificationRule(t *testing.T) {
	tests := []struct {
		name     string
		edges    []Edge
		errT     int
		warnT    int
		wantRule bool
		wantSev  string
	}{
		{
			name: "(1+3)*(1+3)=16 > 10 — error",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 3, Timeout: time.Second},
				{Source: "B", Target: "C", MaxRetries: 3, Timeout: time.Second},
			},
			wantRule: true,
			wantSev:  "error",
		},
		{
			name: "(1+1)*(1+1)=4 < 5 — clean",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 1, Timeout: time.Second},
				{Source: "B", Target: "C", MaxRetries: 1, Timeout: time.Second},
			},
			wantRule: false,
		},
		{
			name: "(1+1)*(1+3)=8 > 5 but <= 10 — warning",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 1, Timeout: time.Second},
				{Source: "B", Target: "C", MaxRetries: 3, Timeout: time.Second},
			},
			wantRule: true,
			wantSev:  "warning",
		},
		{
			name: "custom thresholds — (1+1)*(1+1)=4 > errT 3",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 1, Timeout: time.Second},
				{Source: "B", Target: "C", MaxRetries: 1, Timeout: time.Second},
			},
			errT:     3,
			warnT:    2,
			wantRule: true,
			wantSev:  "error",
		},
		{
			name: "no retries at all — clean",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 0, Timeout: time.Second},
				{Source: "B", Target: "C", MaxRetries: 0, Timeout: time.Second},
			},
			wantRule: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := &RetryAmplificationRule{
				ErrorThreshold:   tc.errT,
				WarningThreshold: tc.warnT,
			}
			g := newMockGraph(tc.edges...)
			vs := rule.Check(g)
			got := hasRule(vs, "retry-amplification")
			if got != tc.wantRule {
				t.Errorf("expected violation=%v, got %v; violations=%+v", tc.wantRule, got, vs)
			}
			if tc.wantRule && tc.wantSev != "" {
				if !hasSeverity(vs, "retry-amplification", tc.wantSev) {
					t.Errorf("expected severity %s; violations=%+v", tc.wantSev, vs)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Rule 3: NonIdempotentRetryRule
// ---------------------------------------------------------------------------

func TestNonIdempotentRetryRule(t *testing.T) {
	tests := []struct {
		name string
		edges []Edge
		want  bool
	}{
		{
			name: "non-idempotent with retries — triggers",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 2, Idempotent: false},
			},
			want: true,
		},
		{
			name: "idempotent with retries — clean",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 2, Idempotent: true},
			},
			want: false,
		},
		{
			name: "non-idempotent with zero retries — clean",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 0, Idempotent: false},
			},
			want: false,
		},
	}

	rule := &NonIdempotentRetryRule{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newMockGraph(tc.edges...)
			vs := rule.Check(g)
			got := hasRule(vs, "non-idempotent-retry")
			if got != tc.want {
				t.Errorf("expected violation=%v, got %v; violations=%+v", tc.want, got, vs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Rule 4: RetryWithoutCircuitBreakerRule
// ---------------------------------------------------------------------------

func TestRetryWithoutCircuitBreakerRule(t *testing.T) {
	tests := []struct {
		name string
		edges []Edge
		want  bool
	}{
		{
			name: "retries without circuit breaker — triggers",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 3, HasCircuitBreaker: false},
			},
			want: true,
		},
		{
			name: "retries with circuit breaker — clean",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 3, HasCircuitBreaker: true},
			},
			want: false,
		},
		{
			name: "no retries without circuit breaker — clean",
			edges: []Edge{
				{Source: "A", Target: "B", MaxRetries: 0, HasCircuitBreaker: false},
			},
			want: false,
		},
	}

	rule := &RetryWithoutCircuitBreakerRule{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newMockGraph(tc.edges...)
			vs := rule.Check(g)
			got := hasRule(vs, "retry-without-cb")
			if got != tc.want {
				t.Errorf("expected violation=%v, got %v; violations=%+v", tc.want, got, vs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Rule 5: BackoffWithoutJitterRule
// ---------------------------------------------------------------------------

func TestBackoffWithoutJitterRule(t *testing.T) {
	tests := []struct {
		name string
		edges []Edge
		want  bool
	}{
		{
			name: "backoff without jitter — triggers",
			edges: []Edge{
				{Source: "A", Target: "B", HasBackoff: true, Jitter: false},
			},
			want: true,
		},
		{
			name: "backoff with jitter — clean",
			edges: []Edge{
				{Source: "A", Target: "B", HasBackoff: true, Jitter: true},
			},
			want: false,
		},
		{
			name: "no backoff and no jitter — clean",
			edges: []Edge{
				{Source: "A", Target: "B", HasBackoff: false, Jitter: false},
			},
			want: false,
		},
	}

	rule := &BackoffWithoutJitterRule{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newMockGraph(tc.edges...)
			vs := rule.Check(g)
			got := hasRule(vs, "backoff-no-jitter")
			if got != tc.want {
				t.Errorf("expected violation=%v, got %v; violations=%+v", tc.want, got, vs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Rule 6: EndToEndTimeoutExceedRule
// ---------------------------------------------------------------------------

func TestEndToEndTimeoutExceedRule(t *testing.T) {
	tests := []struct {
		name         string
		edges        []Edge
		entryTimeout time.Duration
		want         bool
	}{
		{
			// A->B: 2s * (1+2)=6s, B->C: 3s * (1+1)=6s, total=12s > 10s
			name: "worst-case 12s > entry 10s — triggers",
			edges: []Edge{
				{Source: "A", Target: "B", Timeout: 2 * time.Second, MaxRetries: 2},
				{Source: "B", Target: "C", Timeout: 3 * time.Second, MaxRetries: 1},
			},
			entryTimeout: 10 * time.Second,
			want:         true,
		},
		{
			// A->B: 1s * (1+1)=2s, B->C: 1s * (1+1)=2s, total=4s < 10s
			name: "worst-case 4s < entry 10s — clean",
			edges: []Edge{
				{Source: "A", Target: "B", Timeout: 1 * time.Second, MaxRetries: 1},
				{Source: "B", Target: "C", Timeout: 1 * time.Second, MaxRetries: 1},
			},
			entryTimeout: 10 * time.Second,
			want:         false,
		},
		{
			name: "zero entry timeout — always clean",
			edges: []Edge{
				{Source: "A", Target: "B", Timeout: 100 * time.Second, MaxRetries: 10},
			},
			entryTimeout: 0,
			want:         false,
		},
		{
			// A->B: 5s * (1+0)=5s, total=5s < 10s
			name: "single edge within budget — clean",
			edges: []Edge{
				{Source: "A", Target: "B", Timeout: 5 * time.Second, MaxRetries: 0},
			},
			entryTimeout: 10 * time.Second,
			want:         false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := &EndToEndTimeoutExceedRule{EntryTimeout: tc.entryTimeout}
			g := newMockGraph(tc.edges...)
			vs := rule.Check(g)
			got := hasRule(vs, "e2e-timeout-exceed")
			if got != tc.want {
				t.Errorf("expected violation=%v, got %v; violations=%+v", tc.want, got, vs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Verify all rules satisfy the Rule interface at compile time
// ---------------------------------------------------------------------------

var _ Rule = (*TimeoutInversionRule)(nil)
var _ Rule = (*RetryAmplificationRule)(nil)
var _ Rule = (*NonIdempotentRetryRule)(nil)
var _ Rule = (*RetryWithoutCircuitBreakerRule)(nil)
var _ Rule = (*BackoffWithoutJitterRule)(nil)
var _ Rule = (*EndToEndTimeoutExceedRule)(nil)
