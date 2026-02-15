package extractor

import (
	"testing"
)

// ----------- helpers -----------

func findByType(configs []ExtractedConfig, typ string) *ExtractedConfig {
	for i := range configs {
		if configs[i].Type == typ {
			return &configs[i]
		}
	}
	return nil
}

func allByType(configs []ExtractedConfig, typ string) []ExtractedConfig {
	var out []ExtractedConfig
	for _, c := range configs {
		if c.Type == typ {
			out = append(out, c)
		}
	}
	return out
}

// ----------- Tests for sample_client.go -----------

func TestExtractHTTPClientTimeout(t *testing.T) {
	configs, err := ExtractFromFile("testdata/sample_client.go")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	c := findByType(configs, "http-client-timeout")
	if c == nil {
		t.Fatal("expected to find http-client-timeout, got nothing")
	}
	if c.TimeoutMs != 5000 {
		t.Errorf("http-client-timeout: want 5000ms, got %dms", c.TimeoutMs)
	}
	if c.Line != 13 {
		t.Errorf("http-client-timeout: want line 13, got %d", c.Line)
	}
	if c.File != "testdata/sample_client.go" {
		t.Errorf("http-client-timeout: want file testdata/sample_client.go, got %s", c.File)
	}
}

func TestExtractContextWithTimeout(t *testing.T) {
	configs, err := ExtractFromFile("testdata/sample_client.go")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	c := findByType(configs, "context-timeout")
	if c == nil {
		t.Fatal("expected to find context-timeout")
	}
	if c.TimeoutMs != 3000 {
		t.Errorf("context-timeout: want 3000ms, got %dms", c.TimeoutMs)
	}
	if c.Line != 19 {
		t.Errorf("context-timeout: want line 19, got %d", c.Line)
	}
}

func TestExtractGRPCWithTimeout(t *testing.T) {
	configs, err := ExtractFromFile("testdata/sample_client.go")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	c := findByType(configs, "grpc-timeout")
	if c == nil {
		t.Fatal("expected to find grpc-timeout")
	}
	if c.TimeoutMs != 10000 {
		t.Errorf("grpc-timeout: want 10000ms, got %dms", c.TimeoutMs)
	}
	if c.Line != 25 {
		t.Errorf("grpc-timeout: want line 25, got %d", c.Line)
	}
}

func TestExtractRetryDo(t *testing.T) {
	configs, err := ExtractFromFile("testdata/sample_client.go")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	c := findByType(configs, "retry-config")
	if c == nil {
		t.Fatal("expected to find retry-config")
	}
	if c.MaxRetries != 3 {
		t.Errorf("retry-config: want MaxRetries=3, got %d", c.MaxRetries)
	}
	if c.Line != 29 {
		t.Errorf("retry-config: want line 29, got %d", c.Line)
	}
}

func TestExtractAllFromSampleClient(t *testing.T) {
	configs, err := ExtractFromFile("testdata/sample_client.go")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	// We expect exactly 4 configs: http-client-timeout, context-timeout, grpc-timeout, retry-config
	if len(configs) != 4 {
		t.Errorf("want 4 configs, got %d:", len(configs))
		for _, c := range configs {
			t.Logf("  %+v", c)
		}
	}
}

// ----------- Tests for gokit_client.go -----------

func TestExtractGoKitRetry(t *testing.T) {
	configs, err := ExtractFromFile("testdata/gokit_client.go")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	c := findByType(configs, "gokit-retry")
	if c == nil {
		t.Fatal("expected to find gokit-retry")
	}
	if c.MaxRetries != 3 {
		t.Errorf("gokit-retry: want MaxRetries=3, got %d", c.MaxRetries)
	}
	if c.TimeoutMs != 5000 {
		t.Errorf("gokit-retry: want TimeoutMs=5000, got %d", c.TimeoutMs)
	}
	if c.Line != 10 {
		t.Errorf("gokit-retry: want line 10, got %d", c.Line)
	}
}

// ----------- Tests for no_timeout.go -----------

func TestNoTimeoutReturnsEmpty(t *testing.T) {
	configs, err := ExtractFromFile("testdata/no_timeout.go")
	if err != nil {
		t.Fatalf("unexpected error for clean file: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("want 0 configs for no_timeout.go, got %d", len(configs))
		for _, c := range configs {
			t.Logf("  %+v", c)
		}
	}
}

// ----------- Tests for syntax_error.go -----------

func TestSyntaxErrorReturnsError(t *testing.T) {
	_, err := ExtractFromFile("testdata/syntax_error.go")
	if err == nil {
		t.Fatal("expected parse error for syntax_error.go, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// ----------- Tests for non-existent file -----------

func TestNonExistentFileReturnsError(t *testing.T) {
	_, err := ExtractFromFile("testdata/does_not_exist.go")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

// ----------- evalDuration unit tests -----------

func TestEvalDurationFromSource(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		wantMs int64
	}{
		{
			name:   "time.Second alone",
			src:    `package p; import "net/http"; var _ = &http.Client{Timeout: time.Second}`,
			wantMs: 1000,
		},
		{
			name:   "500 * time.Millisecond",
			src:    `package p; import "net/http"; var _ = &http.Client{Timeout: 500 * time.Millisecond}`,
			wantMs: 500,
		},
		{
			name:   "time.Minute",
			src:    `package p; import "net/http"; var _ = &http.Client{Timeout: 2 * time.Minute}`,
			wantMs: 120000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs, err := ExtractFromSource("test.go", []byte(tt.src))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			c := findByType(configs, "http-client-timeout")
			if c == nil {
				t.Fatal("expected http-client-timeout")
			}
			if c.TimeoutMs != tt.wantMs {
				t.Errorf("want %dms, got %dms", tt.wantMs, c.TimeoutMs)
			}
		})
	}
}
