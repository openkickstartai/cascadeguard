package parser

import (
	"fmt"
	"io"
	"time"

	"gopkg.in/yaml.v3"
)

// RawTopology represents the top-level YAML structure for a service topology.
type RawTopology struct {
	Services []RawService `yaml:"services"`
}

// RawService describes a single microservice, its endpoints, and outbound dependencies.
type RawService struct {
	Name         string          `yaml:"name"`
	Endpoints    []RawEndpoint   `yaml:"endpoints"`
	Dependencies []RawDependency `yaml:"dependencies"`
}

// RawEndpoint describes one HTTP endpoint exposed by a service.
type RawEndpoint struct {
	Method     string `yaml:"method"`
	Path       string `yaml:"path"`
	Idempotent bool   `yaml:"idempotent"`
}

// RawDependency describes a call from the owning service to a downstream target.
type RawDependency struct {
	Target                string `yaml:"target"`
	Timeout               string `yaml:"timeout"`
	MaxRetries            int    `yaml:"max_retries"`
	BackoffBase           string `yaml:"backoff_base"`
	BackoffJitterEnabled  bool   `yaml:"backoff_jitter_enabled"`
	CircuitBreakerEnabled bool   `yaml:"circuit_breaker_enabled"`
}

// ParseTopology reads YAML from r, unmarshals it into a RawTopology, and
// validates required fields plus value constraints. An empty reader yields
// an empty (non-nil) topology without error.
func ParseTopology(r io.Reader) (*RawTopology, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}

	var topo RawTopology

	// Empty input is a valid (empty) topology.
	if len(data) == 0 {
		return &topo, nil
	}

	if err := yaml.Unmarshal(data, &topo); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if err := validateTopology(&topo); err != nil {
		return nil, err
	}

	return &topo, nil
}

func validateTopology(topo *RawTopology) error {
	for i, svc := range topo.Services {
		if svc.Name == "" {
			return fmt.Errorf("service[%d]: missing required field \"name\"", i)
		}

		for j, ep := range svc.Endpoints {
			if ep.Method == "" {
				return fmt.Errorf("service %q endpoint[%d]: missing required field \"method\"", svc.Name, j)
			}
			if ep.Path == "" {
				return fmt.Errorf("service %q endpoint[%d]: missing required field \"path\"", svc.Name, j)
			}
		}

		for j, dep := range svc.Dependencies {
			if dep.Target == "" {
				return fmt.Errorf("service %q dependency[%d]: missing required field \"target\"", svc.Name, j)
			}
			if dep.Timeout == "" {
				return fmt.Errorf("service %q dependency[%d] (target %q): missing required field \"timeout\"", svc.Name, j, dep.Target)
			}

			d, err := time.ParseDuration(dep.Timeout)
			if err != nil {
				return fmt.Errorf("service %q dependency[%d] (target %q): invalid timeout %q: %w", svc.Name, j, dep.Target, dep.Timeout, err)
			}
			if d < 0 {
				return fmt.Errorf("service %q dependency[%d] (target %q): timeout must not be negative, got %v", svc.Name, j, dep.Target, d)
			}

			if dep.MaxRetries < 0 {
				return fmt.Errorf("service %q dependency[%d] (target %q): max_retries must not be negative, got %d", svc.Name, j, dep.Target, dep.MaxRetries)
			}

			if dep.BackoffBase != "" {
				if _, err := time.ParseDuration(dep.BackoffBase); err != nil {
					return fmt.Errorf("service %q dependency[%d] (target %q): invalid backoff_base %q: %w", svc.Name, j, dep.Target, dep.BackoffBase, err)
				}
			}
		}
	}
	return nil
}
