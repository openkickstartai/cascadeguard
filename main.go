package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Services map[string]struct {
		Calls []struct {
			Target         string `yaml:"target"`
			Timeout        string `yaml:"timeout"`
			Retries        int    `yaml:"retries"`
			CircuitBreaker bool   `yaml:"circuit_breaker"`
			Method         string `yaml:"method"`
			BackoffJitter  bool   `yaml:"backoff_jitter"`
		} `yaml:"calls"`
	} `yaml:"services"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: cascadeguard <topology.yaml>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(2)
	}
	var edges []CallEdge
	for svc, sc := range cfg.Services {
		for _, c := range sc.Calls {
			t, _ := time.ParseDuration(c.Timeout)
			m := c.Method
			if m == "" {
				m = "GET"
			}
			edges = append(edges, CallEdge{Source: svc, Target: c.Target,
				Timeout: t, Retries: c.Retries, CircuitBreaker: c.CircuitBreaker,
				Method: m, BackoffJitter: c.BackoffJitter})
		}
	}
	g := NewGraph(edges)
	findings := g.Analyze()
	if len(findings) == 0 {
		fmt.Println("No issues found in service topology.")
		return
	}
	fmt.Printf("Found %d issue(s):\n\n", len(findings))
	for i, f := range findings {
		sev := "WARN"
		if f.Severity == "error" {
			sev = "ERR "
		}
		fmt.Printf("%d. [%s][%s] %s\n   Path: %v\n\n", i+1, sev, f.Rule, f.Message, f.Path)
	}
	fmt.Println("--- Mermaid Topology ---")
	fmt.Println("graph LR")
	for _, e := range edges {
		fmt.Printf("  %s -->|\"t=%s r=%d\"| %s\n", e.Source, e.Timeout, e.Retries, e.Target)
	}
	os.Exit(1)
}
