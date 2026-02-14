package parser

import (
	"os"
	"strings"
	"testing"
)

func TestParseValidTopology(t *testing.T) {
	f, err := os.Open("testdata/simple.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	topo, err := ParseTopology(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if topo == nil {
		t.Fatal("expected non-nil topology")
	}
	if len(topo.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(topo.Services))
	}

	// Verify first service
	gw := topo.Services[0]
	if gw.Name != "gateway" {
		t.Errorf("expected service name 'gateway', got %q", gw.Name)
	}
	if len(gw.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint for gateway, got %d", len(gw.Endpoints))
	}
	if gw.Endpoints[0].Method != "GET" {
		t.Errorf("expected method GET, got %q", gw.Endpoints[0].Method)
	}
	if gw.Endpoints[0].Path != "/api/users" {
		t.Errorf("expected path /api/users, got %q", gw.Endpoints[0].Path)
	}
	if !gw.Endpoints[0].Idempotent {
		t.Error("expected idempotent=true for GET /api/users")
	}
	if len(gw.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency for gateway, got %d", len(gw.Dependencies))
	}
	dep := gw.Dependencies[0]
	if dep.Target != "user-svc" {
		t.Errorf("expected target 'user-svc', got %q", dep.Target)
	}
	if dep.Timeout != "3s" {
		t.Errorf("expected timeout '3s', got %q", dep.Timeout)
	}
	if dep.MaxRetries != 2 {
		t.Errorf("expected max_retries 2, got %d", dep.MaxRetries)
	}
	if dep.BackoffBase != "100ms" {
		t.Errorf("expected backoff_base '100ms', got %q", dep.BackoffBase)
	}
	if !dep.BackoffJitterEnabled {
		t.Error("expected backoff_jitter_enabled=true")
	}
	if !dep.CircuitBreakerEnabled {
		t.Error("expected circuit_breaker_enabled=true")
	}

	// Verify second service
	usr := topo.Services[1]
	if usr.Name != "user-svc" {
		t.Errorf("expected service name 'user-svc', got %q", usr.Name)
	}
	if len(usr.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints for user-svc, got %d", len(usr.Endpoints))
	}
	if len(usr.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency for user-svc, got %d", len(usr.Dependencies))
	}
}

func TestParseMissingTimeout(t *testing.T) {
	input := `
services:
  - name: gateway
    dependencies:
      - target: user-svc
        max_retries: 2
`
	_, err := ParseTopology(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("error should mention 'timeout', got: %v", err)
	}
}

func TestParseNegativeTimeout(t *testing.T) {
	input := `
services:
  - name: gateway
    dependencies:
      - target: user-svc
        timeout: "-5s"
        max_retries: 0
`
	_, err := ParseTopology(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
	if !strings.Contains(err.Error(), "negative") {
		t.Fatalf("error should mention 'negative', got: %v", err)
	}
}

func TestParseEmptyFile(t *testing.T) {
	topo, err := ParseTopology(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error for empty input: %v", err)
	}
	if topo == nil {
		t.Fatal("expected non-nil topology for empty input")
	}
	if len(topo.Services) != 0 {
		t.Fatalf("expected 0 services for empty input, got %d", len(topo.Services))
	}
}

func TestParseMalformedYAML(t *testing.T) {
	input := `
services:
  - name: [invalid
    broken: {yaml
`
	_, err := ParseTopology(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "parsing YAML") {
		t.Fatalf("error should indicate a parse error, got: %v", err)
	}
}

func TestParseNegativeMaxRetries(t *testing.T) {
	input := `
services:
  - name: gateway
    dependencies:
      - target: user-svc
        timeout: "3s"
        max_retries: -1
`
	_, err := ParseTopology(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for negative max_retries")
	}
	if !strings.Contains(err.Error(), "max_retries") {
		t.Fatalf("error should mention 'max_retries', got: %v", err)
	}
}

func TestParseMissingServiceName(t *testing.T) {
	input := `
services:
  - endpoints:
      - method: GET
        path: /health
`
	_, err := ParseTopology(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing service name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Fatalf("error should mention 'name', got: %v", err)
	}
}

func TestParseComplexTopology(t *testing.T) {
	f, err := os.Open("testdata/complex.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	topo, err := ParseTopology(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(topo.Services) != 7 {
		t.Fatalf("expected 7 services, got %d", len(topo.Services))
	}
}
