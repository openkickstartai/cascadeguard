package graph

import (
	"testing"
	"time"
)

// --- Test 1: Linear chain A→B→C ---

func TestLinearChain(t *testing.T) {
	g := NewCallGraph()
	g.AddNode(Node{Name: "A"})
	g.AddNode(Node{Name: "B"})
	g.AddNode(Node{Name: "C"})
	g.AddEdge(Edge{From: "A", To: "B", Timeout: 2 * time.Second, MaxRetries: 2})
	g.AddEdge(Edge{From: "B", To: "C", Timeout: 1 * time.Second, MaxRetries: 1})

	paths := g.AllPathsFrom("A")
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if len(paths[0]) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(paths[0]))
	}
	if paths[0][0].From != "A" || paths[0][0].To != "B" {
		t.Errorf("edge 0: want A→B, got %s→%s", paths[0][0].From, paths[0][0].To)
	}
	if paths[0][1].From != "B" || paths[0][1].To != "C" {
		t.Errorf("edge 1: want B→C, got %s→%s", paths[0][1].From, paths[0][1].To)
	}

	// Amplification: (1+2)*(1+1) = 6
	if f := RetryAmplificationFactor(paths[0]); f != 6 {
		t.Errorf("amplification: want 6, got %d", f)
	}
	// Worst-case latency: 2s*3 + 1s*2 = 8s
	if lat := WorstCaseLatency(paths[0]); lat != 8*time.Second {
		t.Errorf("latency: want 8s, got %v", lat)
	}
}

// --- Test 2: Diamond A→B→C + A→D→C ---

func TestDiamond(t *testing.T) {
	g := NewCallGraph()
	for _, n := range []string{"A", "B", "C", "D"} {
		g.AddNode(Node{Name: n})
	}
	g.AddEdge(Edge{From: "A", To: "B", Timeout: time.Second, MaxRetries: 1})
	g.AddEdge(Edge{From: "A", To: "D", Timeout: time.Second, MaxRetries: 1})
	g.AddEdge(Edge{From: "B", To: "C", Timeout: time.Second, MaxRetries: 1})
	g.AddEdge(Edge{From: "D", To: "C", Timeout: time.Second, MaxRetries: 1})

	paths := g.AllPathsFrom("A")
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths through diamond, got %d", len(paths))
	}
	for i, p := range paths {
		if len(p) != 2 {
			t.Errorf("path %d: want 2 edges, got %d", i, len(p))
		}
		if p[0].From != "A" {
			t.Errorf("path %d: first edge should start at A, got %s", i, p[0].From)
		}
		if p[len(p)-1].To != "C" {
			t.Errorf("path %d: last edge should end at C, got %s", i, p[len(p)-1].To)
		}
	}
}

// --- Test 3: Graph with cycle ---

func TestCycleDetection(t *testing.T) {
	g := NewCallGraph()
	g.AddNode(Node{Name: "A"})
	g.AddNode(Node{Name: "B"})
	g.AddNode(Node{Name: "C"})
	g.AddEdge(Edge{From: "A", To: "B", Timeout: time.Second, MaxRetries: 1})
	g.AddEdge(Edge{From: "B", To: "C", Timeout: time.Second, MaxRetries: 1})
	g.AddEdge(Edge{From: "C", To: "A", Timeout: time.Second, MaxRetries: 1}) // back-edge

	paths := g.AllPathsFrom("A")
	if len(paths) != 1 {
		t.Fatalf("expected 1 path (truncated at cycle), got %d", len(paths))
	}
	p := paths[0]
	if len(p) != 3 {
		t.Fatalf("expected 3 edges (incl. cycle-closing edge), got %d", len(p))
	}
	// Last edge should close the cycle back to A.
	if p[2].From != "C" || p[2].To != "A" {
		t.Errorf("cycle edge: want C→A, got %s→%s", p[2].From, p[2].To)
	}
}

// --- Test 4: Single node (no edges, leaf) ---

func TestSingleNode(t *testing.T) {
	g := NewCallGraph()
	g.AddNode(Node{Name: "lonely", Namespace: "default"})

	paths := g.AllPathsFrom("lonely")
	if len(paths) != 0 {
		t.Fatalf("expected 0 paths for isolated node, got %d", len(paths))
	}
}

// --- Test 5: Amplification factor = 1 (no retries) ---

func TestAmplificationFactorNoRetries(t *testing.T) {
	path := []Edge{
		{From: "A", To: "B", Timeout: time.Second, MaxRetries: 0},
		{From: "B", To: "C", Timeout: time.Second, MaxRetries: 0},
		{From: "C", To: "D", Timeout: time.Second, MaxRetries: 0},
	}
	if f := RetryAmplificationFactor(path); f != 1 {
		t.Errorf("want amplification factor 1 (no retries), got %d", f)
	}
}

// --- Test 6: Amplification factor > 10 (deep chain) ---

func TestAmplificationFactorDeepChain(t *testing.T) {
	// 5 edges × MaxRetries=2 → (1+2)^5 = 243
	names := []string{"A", "B", "C", "D", "E", "F"}
	var path []Edge
	for i := 0; i < len(names)-1; i++ {
		path = append(path, Edge{
			From:       names[i],
			To:         names[i+1],
			Timeout:    time.Second,
			MaxRetries: 2,
		})
	}
	f := RetryAmplificationFactor(path)
	if f != 243 {
		t.Errorf("want amplification factor 243, got %d", f)
	}
	if f <= 10 {
		t.Errorf("amplification factor %d should be > 10 for deep retry chain", f)
	}
}

// --- Test 7: Zero timeout edge ---

func TestZeroTimeoutEdge(t *testing.T) {
	path := []Edge{
		{From: "A", To: "B", Timeout: 2 * time.Second, MaxRetries: 1},
		{From: "B", To: "C", Timeout: 0, MaxRetries: 3},
	}
	// Latency: 2s*(1+1) + 0*(1+3) = 4s
	lat := WorstCaseLatency(path)
	if lat != 4*time.Second {
		t.Errorf("want 4s latency, got %v", lat)
	}

	// Amplification should still work: (1+1)*(1+3) = 8
	if f := RetryAmplificationFactor(path); f != 8 {
		t.Errorf("want amplification factor 8, got %d", f)
	}
}

// --- Test 8: Empty graph ---

func TestEmptyGraph(t *testing.T) {
	g := NewCallGraph()

	paths := g.AllPathsFrom("ghost")
	if len(paths) != 0 {
		t.Fatalf("expected 0 paths for empty graph, got %d", len(paths))
	}

	// Edge-case: empty/nil path.
	if f := RetryAmplificationFactor(nil); f != 1 {
		t.Errorf("want factor 1 for nil path, got %d", f)
	}
	if lat := WorstCaseLatency(nil); lat != 0 {
		t.Errorf("want 0 latency for nil path, got %v", lat)
	}
}

// --- Additional: self-loop (tightest cycle) ---

func TestSelfLoop(t *testing.T) {
	g := NewCallGraph()
	g.AddNode(Node{Name: "X"})
	g.AddEdge(Edge{From: "X", To: "X", Timeout: 500 * time.Millisecond, MaxRetries: 4})

	paths := g.AllPathsFrom("X")
	if len(paths) != 1 {
		t.Fatalf("expected 1 path for self-loop, got %d", len(paths))
	}
	if len(paths[0]) != 1 {
		t.Fatalf("expected 1 edge in self-loop path, got %d", len(paths[0]))
	}
	if paths[0][0].From != "X" || paths[0][0].To != "X" {
		t.Errorf("self-loop edge: want X→X, got %s→%s", paths[0][0].From, paths[0][0].To)
	}

	// Latency: 500ms * (1+4) = 2.5s
	if lat := WorstCaseLatency(paths[0]); lat != 2500*time.Millisecond {
		t.Errorf("want 2.5s, got %v", lat)
	}
}

// --- Additional: mixed cycle and non-cycle branches ---

func TestMixedCycleAndLeaf(t *testing.T) {
	// A → B → A (cycle)
	// A → C      (leaf)
	g := NewCallGraph()
	g.AddNode(Node{Name: "A"})
	g.AddNode(Node{Name: "B"})
	g.AddNode(Node{Name: "C"})
	g.AddEdge(Edge{From: "A", To: "B", Timeout: time.Second, MaxRetries: 0})
	g.AddEdge(Edge{From: "A", To: "C", Timeout: time.Second, MaxRetries: 0})
	g.AddEdge(Edge{From: "B", To: "A", Timeout: time.Second, MaxRetries: 0})

	paths := g.AllPathsFrom("A")
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths (1 cycle + 1 leaf), got %d", len(paths))
	}

	// One path should be [A→B, B→A] (cycle), the other [A→C] (leaf).
	var hasCycle, hasLeaf bool
	for _, p := range paths {
		if len(p) == 2 && p[0].To == "B" && p[1].To == "A" {
			hasCycle = true
		}
		if len(p) == 1 && p[0].To == "C" {
			hasLeaf = true
		}
	}
	if !hasCycle {
		t.Error("missing cycle path A→B→A")
	}
	if !hasLeaf {
		t.Error("missing leaf path A→C")
	}
}
