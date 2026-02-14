package parser

import (
	"strings"
	"testing"
)

func TestParseValidTopology(t *testing.T) {
	input := `
services:
  - name: gateway
    endpoints:
      - method: GET
        path: /users
        idempotent: true
      - method: POST
        path: /users
        idempotent: false
    dependencies:
      - target: user-svc
        timeout: 3s
        max_retries: 2
        backoff_base: 100ms
        backoff_jitter_enabled: true
        circuit_breaker_enabled: true
  - name: user-svc
    endpoints:
      - method: GET
        path: /internal/users
        idempotent: true
    dependencies:
      - target: db-svc
        timeout: 1s
        max_retries: 1
        backoff_base: 50ms
        backoff_jitter_enabled: false
        circuit_breaker_enabled: false
`
	topo, err := ParseTopology(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(topo.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(topo.Services))
	}

	gw := topo.Services[0]
	if gw.Name != "gateway" {
		t.Errorf("expected service name 'gateway', got %q", gw.Name)
	}
	if len(gw.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints on gateway, got %d", len(gw.Endpoints))
	}
	if gw.Endpoints[0].Method != "GET" || gw.Endpoints[0].Path != "/users" || !gw.Endpoints[0].Idempotent {
		t.Errorf("endpoint[0] mismatch: %+v", gw.Endpoints[0])
	}
	if gw.Endpoints[1].Method != "POST" || gw.Endpoints[1].Path != "/users" || gw.Endpoints[1].Idempotent {
		t.Errorf("endpoint[1] mismatch: %+v", gw.Endpoints[1])
	}

	if len(gw.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency on gateway, got %d", len(gw.Dependencies))
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
		t.Error("expected backoff_jitter_enabled true")
	}
	if !dep.CircuitBreakerEnabled {
		t.Error("expected circuit_breaker_enabled true")
	}

	usr := topo.Services[1]
	if usr.Name != "user-svc" {
		t.Errorf("expected service name 'user-svc', got %q", usr.Name)
	}
	if len(usr.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency on user-svc, got %d", len(usr.Dependencies))
	}
	if usr.Dependencies[0].Target != "db-svc" {
		t.Errorf("expected target 'db-svc', got %q", usr.Dependencies[0].Target)
	}
}

func TestMissingTimeoutField(t *testing.T) {
	input := `
services:
  - name: gateway
    dependencies:
      - target: user-svc
        max_retries: 2
`
	_, err := ParseTopology(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing timeout field")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("error message should contain 'timeout', got: %v", err)
	}
}

func TestNegativeTimeout(t *testing.T) {
	input := `
services:
  - name: gateway
    dependencies:
      - target: user-svc
        timeout: -3s
        max_retries: 0
`
	_, err := ParseTopology(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
	if !strings.Contains(err.Error(), "negative") {
		t.Fatalf("error message should contain 'negative', got: %v", err)
	}
}

func TestEmptyFileReturnsEmptyTopology(t *testing.T) {
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

func TestMalformedYAML(t *testing.T) {
	input := `{{{not valid yaml:::`
	_, err := ParseTopology(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestNegativeMaxRetries(t *testing.T) {
	input := `
services:
  - name: gateway
    dependencies:
      - target: user-svc
        timeout: 3s
        max_retries: -1
`
	_, err := ParseTopology(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for negative max_retries")
	}
	if !strings.Contains(err.Error(), "max_retries") {
		t.Fatalf("error message should contain 'max_retries', got: %v", err)
	}
	if !strings.Contains(err.Error(), "negative") {
		t.Fatalf("error message should contain 'negative', got: %v", err)
	}
}
