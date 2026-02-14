package main

import (
	"fmt"
	"time"
)

type CallEdge struct {
	Source, Target string
	Timeout        time.Duration
	Retries        int
	CircuitBreaker bool
	Method         string
	BackoffJitter  bool
}

type Finding struct {
	Rule, Severity, Message string
	Path                    []string
}

type Graph struct {
	Edges []CallEdge
	Adj   map[string][]CallEdge
}

func NewGraph(edges []CallEdge) *Graph {
	adj := make(map[string][]CallEdge)
	for _, e := range edges {
		adj[e.Source] = append(adj[e.Source], e)
	}
	return &Graph{Edges: edges, Adj: adj}
}

func (g *Graph) Analyze() []Finding {
	var f []Finding
	f = append(f, g.edgeRules()...)
	f = append(f, g.amplification()...)
	return f
}

func (g *Graph) edgeRules() []Finding {
	var f []Finding
	nonIdem := map[string]bool{"POST": true, "PATCH": true, "DELETE": true}
	for _, e := range g.Edges {
		p := []string{e.Source, e.Target}
		for _, d := range g.Adj[e.Target] {
			if e.Timeout > 0 && d.Timeout > e.Timeout {
				f = append(f, Finding{"timeout-inversion", "error", fmt.Sprintf(
					"%s->%s timeout %v but %s->%s timeout %v (downstream > upstream)",
					e.Source, e.Target, e.Timeout, e.Target, d.Target, d.Timeout),
					[]string{e.Source, e.Target, d.Target}})
			}
		}
		if e.Retries > 0 && !e.CircuitBreaker {
			f = append(f, Finding{"retry-without-cb", "warning", fmt.Sprintf(
				"%s->%s has %d retries but no circuit breaker", e.Source, e.Target, e.Retries), p})
		}
		if e.Retries > 0 && nonIdem[e.Method] {
			f = append(f, Finding{"non-idempotent-retry", "error", fmt.Sprintf(
				"%s->%s retries %s %d times (non-idempotent)", e.Source, e.Target, e.Method, e.Retries), p})
		}
		if e.Retries > 0 && !e.BackoffJitter {
			f = append(f, Finding{"backoff-no-jitter", "warning", fmt.Sprintf(
				"%s->%s retries without jitter (thundering herd risk)", e.Source, e.Target), p})
		}
	}
	return f
}

func (g *Graph) amplification() []Finding {
	var f []Finding
	incoming := map[string]bool{}
	for _, e := range g.Edges {
		incoming[e.Target] = true
	}
	for src := range g.Adj {
		if !incoming[src] {
			g.dfs(src, []string{src}, 1, &f)
		}
	}
	return f
}

func (g *Graph) dfs(node string, path []string, factor int, f *[]Finding) {
	for _, e := range g.Adj[node] {
		af := factor * (1 + e.Retries)
		np := append(append([]string{}, path...), e.Target)
		if af > 10 {
			*f = append(*f, Finding{"retry-amplification", "error", fmt.Sprintf(
				"amplification factor %dx along path (threshold 10x)", af), np})
		}
		if len(np) < 10 {
			g.dfs(e.Target, np, af, f)
		}
	}
}
