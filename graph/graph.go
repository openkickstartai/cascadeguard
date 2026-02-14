package graph

import "time"

// BackoffConfig holds backoff parameters for retry policies.
type BackoffConfig struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	HasJitter       bool
}

// Node represents a service in the call graph.
type Node struct {
	Name      string
	Namespace string
}

// Edge represents a directed call between two services.
type Edge struct {
	From              string
	To                string
	Timeout           time.Duration
	MaxRetries        int
	Backoff           BackoffConfig
	HasCircuitBreaker bool
	Idempotent        bool
}

// CallGraph is a directed graph of service-to-service calls.
type CallGraph struct {
	nodes map[string]Node
	adj   map[string][]Edge
}

// NewCallGraph creates an empty CallGraph.
func NewCallGraph() *CallGraph {
	return &CallGraph{
		nodes: make(map[string]Node),
		adj:   make(map[string][]Edge),
	}
}

// AddNode registers a service node in the graph.
func (g *CallGraph) AddNode(n Node) {
	g.nodes[n.Name] = n
}

// AddEdge adds a directed call edge to the graph.
func (g *CallGraph) AddEdge(e Edge) {
	g.adj[e.From] = append(g.adj[e.From], e)
}

// AllPathsFrom enumerates every path from root to a leaf node using DFS.
// Cycles are detected: when a back-edge is found the path is truncated and
// the cycle-closing edge is included so callers can identify the loop
// (its To field will match an earlier From in the path or the root itself).
func (g *CallGraph) AllPathsFrom(root string) [][]Edge {
	var result [][]Edge
	visited := make(map[string]bool)
	g.dfs(root, visited, nil, &result)
	return result
}

func (g *CallGraph) dfs(node string, visited map[string]bool, path []Edge, result *[][]Edge) {
	visited[node] = true
	defer func() { visited[node] = false }()

	outEdges := g.adj[node]
	if len(outEdges) == 0 {
		// Leaf node: record current path if non-empty.
		if len(path) > 0 {
			cp := make([]Edge, len(path))
			copy(cp, path)
			*result = append(*result, cp)
		}
		return
	}

	for _, e := range outEdges {
		newPath := make([]Edge, len(path)+1)
		copy(newPath, path)
		newPath[len(path)] = e

		if visited[e.To] {
			// Cycle detected: include the back-edge but stop recursion.
			*result = append(*result, newPath)
			continue
		}
		g.dfs(e.To, visited, newPath, result)
	}
}

// RetryAmplificationFactor returns the multiplicative retry factor along a
// path. Each edge contributes (1 + MaxRetries) attempts; the product gives
// the worst-case total number of leaf requests triggered by one root request.
// Returns 1 for an empty path.
func RetryAmplificationFactor(path []Edge) int {
	if len(path) == 0 {
		return 1
	}
	factor := 1
	for _, e := range path {
		factor *= 1 + e.MaxRetries
	}
	return factor
}

// WorstCaseLatency computes the worst-case end-to-end latency along a path.
// Each edge contributes Timeout × (1 + MaxRetries). Returns 0 for an empty path.
func WorstCaseLatency(path []Edge) time.Duration {
	var total time.Duration
	for _, e := range path {
		total += e.Timeout * time.Duration(1+e.MaxRetries)
	}
	return total
}
