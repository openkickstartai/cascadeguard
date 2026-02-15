package extractor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// ExtractedConfig holds a single timeout or retry configuration found in source.
type ExtractedConfig struct {
	File       string
	Line       int
	Type       string // e.g. "http-client-timeout", "context-timeout", "grpc-timeout", "retry-config", "gokit-retry"
	TimeoutMs  int64
	MaxRetries int
}

// ExtractFromFile parses a Go source file and extracts timeout/retry configs.
func ExtractFromFile(filename string) ([]ExtractedConfig, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	var configs []ExtractedConfig

	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			if cfg := matchHTTPClient(fset, filename, node); cfg != nil {
				configs = append(configs, *cfg)
			}
		case *ast.CallExpr:
			configs = append(configs, matchCallExpr(fset, filename, node)...)
		}
		return true
	})

	return configs, nil
}

// ExtractFromSource parses Go source bytes (useful for testing without files).
func ExtractFromSource(filename string, src []byte) ([]ExtractedConfig, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	var configs []ExtractedConfig

	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			if cfg := matchHTTPClient(fset, filename, node); cfg != nil {
				configs = append(configs, *cfg)
			}
		case *ast.CallExpr:
			configs = append(configs, matchCallExpr(fset, filename, node)...)
		}
		return true
	})

	return configs, nil
}

// matchHTTPClient detects &http.Client{Timeout: <expr>} or http.Client{Timeout: <expr>}.
func matchHTTPClient(fset *token.FileSet, filename string, cl *ast.CompositeLit) *ExtractedConfig {
	if !isSel(cl.Type, "http", "Client") {
		return nil
	}
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Timeout" {
			continue
		}
		return &ExtractedConfig{
			File:      filename,
			Line:      fset.Position(cl.Pos()).Line,
			Type:      "http-client-timeout",
			TimeoutMs: evalDuration(kv.Value),
		}
	}
	return nil
}

// matchCallExpr detects context.WithTimeout, grpc.WithTimeout, retry.Do, and go-kit Retry.
func matchCallExpr(fset *token.FileSet, filename string, call *ast.CallExpr) []ExtractedConfig {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	pkg := x.Name
	fn := sel.Sel.Name
	line := fset.Position(call.Pos()).Line

	var out []ExtractedConfig

	switch {
	// context.WithTimeout(ctx, duration)
	case pkg == "context" && fn == "WithTimeout" && len(call.Args) >= 2:
		out = append(out, ExtractedConfig{
			File:      filename,
			Line:      line,
			Type:      "context-timeout",
			TimeoutMs: evalDuration(call.Args[1]),
		})

	// grpc.WithTimeout(duration)
	case pkg == "grpc" && fn == "WithTimeout" && len(call.Args) >= 1:
		out = append(out, ExtractedConfig{
			File:      filename,
			Line:      line,
			Type:      "grpc-timeout",
			TimeoutMs: evalDuration(call.Args[0]),
		})

	// retry.Do(fn, retry.Attempts(N), ...)
	case pkg == "retry" && fn == "Do":
		cfg := ExtractedConfig{
			File: filename,
			Line: line,
			Type: "retry-config",
		}
		for _, arg := range call.Args {
			ac, ok := arg.(*ast.CallExpr)
			if !ok {
				continue
			}
			as, ok := ac.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			ax, ok := as.X.(*ast.Ident)
			if !ok {
				continue
			}
			if ax.Name == "retry" && as.Sel.Name == "Attempts" && len(ac.Args) >= 1 {
				cfg.MaxRetries = evalInt(ac.Args[0])
			}
		}
		out = append(out, cfg)

	// go-kit: lb.Retry(maxRetries, timeout, ...) or sd.Retry(...)
	case (pkg == "lb" || pkg == "sd") && fn == "Retry" && len(call.Args) >= 2:
		out = append(out, ExtractedConfig{
			File:       filename,
			Line:       line,
			Type:       "gokit-retry",
			MaxRetries: evalInt(call.Args[0]),
			TimeoutMs:  evalDuration(call.Args[1]),
		})
	}

	return out
}

// ---------------------------------------------------------------------------
// Duration / integer evaluation helpers
// ---------------------------------------------------------------------------

// evalDuration attempts to compute a millisecond value from a duration
// expression like `5 * time.Second` or `time.Millisecond * 200`.
func evalDuration(expr ast.Expr) int64 {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		if e.Op == token.MUL {
			l := evalDuration(e.X)
			r := evalDuration(e.Y)
			if l != 0 && r != 0 {
				return l * r
			}
		}
		if e.Op == token.ADD {
			return evalDuration(e.X) + evalDuration(e.Y)
		}
	case *ast.SelectorExpr:
		return timeConstMs(e)
	case *ast.BasicLit:
		if e.Kind == token.INT {
			v, _ := strconv.ParseInt(e.Value, 10, 64)
			return v
		}
	case *ast.ParenExpr:
		return evalDuration(e.X)
	case *ast.CallExpr:
		// Handle casts like time.Duration(n)
		if len(e.Args) == 1 {
			return evalDuration(e.Args[0])
		}
	}
	return 0
}

// timeConstMs maps time.XYZ selector expressions to milliseconds.
func timeConstMs(sel *ast.SelectorExpr) int64 {
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Name != "time" {
		return 0
	}
	switch sel.Sel.Name {
	case "Nanosecond":
		return 0 // < 1ms, return 0
	case "Microsecond":
		return 0 // < 1ms, return 0
	case "Millisecond":
		return 1
	case "Second":
		return 1000
	case "Minute":
		return 60_000
	case "Hour":
		return 3_600_000
	}
	return 0
}

// evalInt extracts an integer literal value, handling simple casts.
func evalInt(expr ast.Expr) int {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.INT {
			v, _ := strconv.Atoi(e.Value)
			return v
		}
	case *ast.CallExpr:
		// Handle uint(3), int(3), etc.
		if len(e.Args) == 1 {
			return evalInt(e.Args[0])
		}
	}
	return 0
}

// isSel checks if an expression is a selector of the form pkg.Name.
func isSel(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return x.Name == pkg && sel.Sel.Name == name
}
