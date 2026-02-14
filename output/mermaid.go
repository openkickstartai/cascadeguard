package output

import (
	"fmt"
	"io"
)

// Edge represents a call edge in the service topology.
type Edge struct {
	Source  string
	Target  string
	Timeout string
	Retries int
}

// CallGraph represents the service call graph.
type CallGraph struct {
	Edges []Edge
}

// Violation represents a detected anti-pattern.
type Violation struct {
	Rule     string
	Severity string
	Message  string
	Path     []string
}

// RenderMermaid writes a Mermaid flowchart to w.
// Edge labels use the format "timeout/retries".
// Edges involved in violations are styled red via linkStyle directives.
// The output ends without extra blank lines.
func RenderMermaid(graph CallGraph, violations []Violation, w io.Writer) error {
	type edgeKey struct{ src, tgt string }
	violationEdges := make(map[edgeKey]bool)
	for _, v := range violations {
		for i := 0; i+1 < len(v.Path); i++ {
			violationEdges[edgeKey{v.Path[i], v.Path[i+1]}] = true
		}
	}

	// Collect all output lines to avoid trailing blank lines.
	lines := make([]string, 0, len(graph.Edges)+2)
	lines = append(lines, "graph LR")

	var redIndices []int
	for i, e := range graph.Edges {
		lines = append(lines, fmt.Sprintf("  %s -->|\"%s/%d\"| %s", e.Source, e.Timeout, e.Retries, e.Target))
		if violationEdges[edgeKey{e.Source, e.Target}] {
			redIndices = append(redIndices, i)
		}
	}

	for _, idx := range redIndices {
		lines = append(lines, fmt.Sprintf("  linkStyle %d stroke:red", idx))
	}

	for i, line := range lines {
		if i > 0 {
			if _, err := fmt.Fprint(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, line); err != nil {
			return err
		}
	}

	return nil
}
