package output

import (
	"encoding/json"
	"io"
)

const sarifSchemaURL = "https://json.schemastore.org/sarif-2.1.0.json"

type sarifDocument struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool      `json:"tool"`
	Results []sarifResult  `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name string `json:"name"`
}

type sarifResult struct {
	RuleID  string       `json:"ruleId"`
	Level   string       `json:"level"`
	Message sarifMessage `json:"message"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

// RenderSARIF writes a SARIF v2.1.0 JSON document to w.
// Each Violation is mapped to a SARIF result. Severity is mapped to SARIF
// level: "error" → "error", "warning" → "warning", anything else → "note".
// The tool driver name is "CascadeGuard".
func RenderSARIF(violations []Violation, w io.Writer) error {
	results := make([]sarifResult, 0, len(violations))
	for _, v := range violations {
		level := mapLevel(v.Severity)
		results = append(results, sarifResult{
			RuleID:  v.Rule,
			Level:   level,
			Message: sarifMessage{Text: v.Message},
		})
	}

	doc := sarifDocument{
		Schema:  sarifSchemaURL,
		Version: "2.1.0",
		Runs: []sarifRun{
			{
				Tool: sarifTool{
					Driver: sarifDriver{Name: "CascadeGuard"},
				},
				Results: results,
			},
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func mapLevel(severity string) string {
	switch severity {
	case "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "note"
	}
}
