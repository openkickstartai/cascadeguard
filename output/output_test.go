package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMermaidStartsWithGraphLR(t *testing.T) {
	graph := CallGraph{
		Edges: []Edge{
			{Source: "A", Target: "B", Timeout: "3s", Retries: 3},
			{Source: "B", Target: "C", Timeout: "5s", Retries: 2},
		},
	}
	var buf bytes.Buffer
	if err := RenderMermaid(graph, nil, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "graph LR") {
		t.Fatalf("expected output to start with 'graph LR', got:\n%s", out)
	}
}

func TestMermaidViolationEdgeStrokeRed(t *testing.T) {
	graph := CallGraph{
		Edges: []Edge{
			{Source: "A", Target: "B", Timeout: "3s", Retries: 3},
			{Source: "B", Target: "C", Timeout: "5s", Retries: 2},
		},
	}
	violations := []Violation{
		{
			Rule:     "timeout-inversion",
			Severity: "error",
			Message:  "downstream timeout exceeds upstream",
			Path:     []string{"A", "B", "C"},
		},
	}
	var buf bytes.Buffer
	if err := RenderMermaid(graph, violations, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "stroke:red") {
		t.Fatalf("expected 'stroke:red' for violation edge, got:\n%s", out)
	}
}

func TestMermaidNoViolationNoRed(t *testing.T) {
	graph := CallGraph{
		Edges: []Edge{
			{Source: "A", Target: "B", Timeout: "3s", Retries: 1},
		},
	}
	var buf bytes.Buffer
	if err := RenderMermaid(graph, nil, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "stroke:red") {
		t.Fatalf("expected no 'stroke:red' without violations, got:\n%s", out)
	}
}

func TestMermaidEdgeLabelFormat(t *testing.T) {
	graph := CallGraph{
		Edges: []Edge{
			{Source: "gateway", Target: "user-svc", Timeout: "3s", Retries: 3},
		},
	}
	var buf bytes.Buffer
	if err := RenderMermaid(graph, nil, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "3s/3") {
		t.Fatalf("expected edge label format 'timeout/retries', got:\n%s", out)
	}
}

func TestMermaidNoExtraTrailingBlankLines(t *testing.T) {
	graph := CallGraph{
		Edges: []Edge{
			{Source: "A", Target: "B", Timeout: "1s", Retries: 1},
		},
	}
	var buf bytes.Buffer
	if err := RenderMermaid(graph, nil, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.HasSuffix(out, "\n\n") {
		t.Fatal("output has extra trailing blank line")
	}
}

func TestSARIFValidJSON(t *testing.T) {
	violations := []Violation{
		{
			Rule:     "timeout-inversion",
			Severity: "error",
			Message:  "downstream timeout exceeds upstream",
			Path:     []string{"A", "B", "C"},
		},
	}
	var buf bytes.Buffer
	if err := RenderSARIF(violations, &buf); err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	schema, ok := m["$schema"].(string)
	if !ok {
		t.Fatal("$schema field missing or not a string")
	}
	if !strings.Contains(strings.ToLower(schema), "sarif") {
		t.Fatalf("$schema should point to SARIF schema URL, got: %s", schema)
	}
}

func TestSARIFEmptyViolations(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderSARIF([]Violation{}, &buf); err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	runs, ok := m["runs"].([]interface{})
	if !ok || len(runs) == 0 {
		t.Fatal("expected non-empty runs array")
	}
	run, ok := runs[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected run to be an object")
	}
	results, ok := run["results"].([]interface{})
	if !ok {
		t.Fatal("expected results to be an array")
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results array, got %d elements", len(results))
	}
}

func TestSARIFToolDriverName(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderSARIF([]Violation{}, &buf); err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	runs := m["runs"].([]interface{})
	run := runs[0].(map[string]interface{})
	tool := run["tool"].(map[string]interface{})
	driver := tool["driver"].(map[string]interface{})
	name, ok := driver["name"].(string)
	if !ok || name != "CascadeGuard" {
		t.Fatalf("expected tool.driver.name='CascadeGuard', got: %v", driver["name"])
	}
}

func TestSARIFSeverityMapping(t *testing.T) {
	violations := []Violation{
		{Rule: "r1", Severity: "error", Message: "err msg", Path: []string{"A", "B"}},
		{Rule: "r2", Severity: "warning", Message: "warn msg", Path: []string{"C", "D"}},
	}
	var buf bytes.Buffer
	if err := RenderSARIF(violations, &buf); err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	runs := m["runs"].([]interface{})
	run := runs[0].(map[string]interface{})
	results := run["results"].([]interface{})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	r0 := results[0].(map[string]interface{})
	if r0["level"] != "error" {
		t.Fatalf("expected level 'error', got %v", r0["level"])
	}
	if r0["ruleId"] != "r1" {
		t.Fatalf("expected ruleId 'r1', got %v", r0["ruleId"])
	}

	r1 := results[1].(map[string]interface{})
	if r1["level"] != "warning" {
		t.Fatalf("expected level 'warning', got %v", r1["level"])
	}
}

func TestSARIFVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderSARIF([]Violation{}, &buf); err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	version, ok := m["version"].(string)
	if !ok || version != "2.1.0" {
		t.Fatalf("expected version '2.1.0', got: %v", m["version"])
	}
}
