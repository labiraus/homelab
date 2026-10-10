package worldbuilder

import "strings"

// Advisory findings never constitute approval or deterministic verification.
type AdvisoryFinding struct {
	Severity    string   `json:"severity"`
	Uncertainty string   `json:"uncertainty"`
	Message     string   `json:"message"`
	Citations   []string `json:"citations"`
}
type ReviewContext struct {
	ChangeID     string      `json:"changeId"`
	BaseRevision int64       `json:"baseRevision"`
	Operations   []Operation `json:"operations"`
}
type DeterministicCheck struct {
	Passed   bool   `json:"passed"`
	Revision int64  `json:"revision"`
	ChangeID string `json:"changeId,omitempty"`
}

func validateFindings(workflow string, findings []AdvisoryFinding, allowed map[string]bool) error {
	if len(findings) > 20 || (len(findings) > 0 && workflow != "consistency_check") {
		return bad("advisory findings require a bounded consistency check")
	}
	for _, f := range findings {
		if (f.Severity != "info" && f.Severity != "warning" && f.Severity != "critical") || (f.Uncertainty != "low" && f.Uncertainty != "medium" && f.Uncertainty != "high") || strings.TrimSpace(f.Message) == "" || len(f.Message) > 2000 || len(f.Citations) == 0 || len(f.Citations) > 10 {
			return bad("invalid advisory finding")
		}
		seen := map[string]bool{}
		for _, id := range f.Citations {
			if !allowed[id] || seen[id] {
				return bad("advisory finding has unresolvable or duplicate evidence")
			}
			seen[id] = true
		}
	}
	return nil
}
