package classify

import (
	"testing"

	"github.com/mistakeknot/interserve/internal/extract"
)

func TestClassifyLocalSafetySection(t *testing.T) {
	sections := []extract.Section{
		{ID: 1, Heading: "Security Audit", Body: "Check credentials and access control for injection vulnerabilities.", LineCount: 5},
	}
	agents := DefaultAgents()

	classified := ClassifyLocal(sections, agents)

	assignments, ok := classified[1]
	if !ok {
		t.Fatal("expected section 1 to have assignments")
	}

	found := false
	for _, a := range assignments {
		if a.Agent == "fd-safety" {
			found = true
			if a.Relevance != "priority" {
				t.Errorf("expected fd-safety relevance=priority, got %q", a.Relevance)
			}
			if a.Confidence < 0.6 {
				t.Errorf("expected fd-safety confidence >= 0.6, got %f", a.Confidence)
			}
		}
	}
	if !found {
		t.Errorf("expected fd-safety assignment for security section, got: %+v", assignments)
	}
}

func TestClassifyLocalCorrectnessSection(t *testing.T) {
	sections := []extract.Section{
		{ID: 1, Heading: "Concurrency Model", Body: "Uses mutex locks to prevent race conditions in goroutine pools.", LineCount: 3},
	}

	classified := ClassifyLocal(sections, DefaultAgents())

	assignments := classified[1]
	found := false
	for _, a := range assignments {
		if a.Agent == "fd-correctness" && a.Relevance == "priority" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected fd-correctness priority assignment, got: %+v", assignments)
	}
}

func TestClassifyLocalPerformanceSection(t *testing.T) {
	sections := []extract.Section{
		{ID: 1, Heading: "Caching Strategy", Body: "LRU cache with connection pooling for latency reduction.", LineCount: 2},
	}

	classified := ClassifyLocal(sections, DefaultAgents())

	assignments := classified[1]
	found := false
	for _, a := range assignments {
		if a.Agent == "fd-performance" && a.Relevance == "priority" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected fd-performance priority assignment, got: %+v", assignments)
	}
}

func TestClassifyLocalLowMatchSection(t *testing.T) {
	// Generic section with no domain-specific keywords should get
	// at most low-confidence context assignments.
	sections := []extract.Section{
		{ID: 1, Heading: "Acknowledgements", Body: "Thanks to all contributors who helped with this project.", LineCount: 1},
	}

	classified := ClassifyLocal(sections, DefaultAgents())

	for _, a := range classified[1] {
		if a.Relevance == "priority" {
			t.Errorf("generic section should not get priority assignment: %+v", a)
		}
		if a.Confidence > 0.6 {
			t.Errorf("generic section should not get high confidence: %+v", a)
		}
	}
}

func TestClassifyLocalCrossCuttingAgents(t *testing.T) {
	sections := []extract.Section{
		{ID: 1, Heading: "Architecture Overview", Body: "Module boundaries and dependency layers define the coupling between components.", LineCount: 4},
	}

	classified := ClassifyLocal(sections, DefaultAgents())

	assignments := classified[1]
	found := false
	for _, a := range assignments {
		if a.Agent == "fd-architecture" {
			found = true
			if a.Relevance != "priority" {
				t.Errorf("expected fd-architecture relevance=priority, got %q", a.Relevance)
			}
		}
	}
	if !found {
		t.Errorf("expected fd-architecture assignment, got: %+v", assignments)
	}
}

func TestClassifyLocalMultipleAgents(t *testing.T) {
	sections := []extract.Section{
		{ID: 1, Heading: "Database Security", Body: "SQL injection prevention with parameterized queries and transaction isolation levels.", LineCount: 3},
	}

	classified := ClassifyLocal(sections, DefaultAgents())
	assignments := classified[1]

	agentNames := make(map[string]bool)
	for _, a := range assignments {
		agentNames[a.Agent] = true
	}

	if !agentNames["fd-safety"] {
		t.Error("expected fd-safety for SQL injection content")
	}
	if !agentNames["fd-correctness"] {
		t.Error("expected fd-correctness for transaction content")
	}
}

func TestClassifyLocalIntegrationWithBuildResult(t *testing.T) {
	sections := []extract.Section{
		{ID: 1, Heading: "Authentication", Body: "Token-based auth with encrypted credentials and access control policies.", LineCount: 80},
		{ID: 2, Heading: "Helpers", Body: "Utility functions for string manipulation and date formatting.", LineCount: 20},
	}
	agents := DefaultAgents()

	classified := ClassifyLocal(sections, agents)
	result := buildResult(classified, sections, agents)

	if result.Status != "success" && result.Status != "no_classification" {
		t.Fatalf("unexpected status: %q, error: %s", result.Status, result.Error)
	}

	if len(result.Sections) != 2 {
		t.Fatalf("expected 2 sections in result, got %d", len(result.Sections))
	}
}

func TestScoreRelevance(t *testing.T) {
	tests := []struct {
		name            string
		priorityHits    int
		contextHits     int
		totalPriority   int
		totalContext    int
		wantRelevance   string
		wantMinConf     float64
		wantMaxConf     float64
	}{
		{"strong priority", 3, 1, 10, 5, "priority", 0.70, 0.95},
		{"single priority + context", 1, 2, 10, 5, "priority", 0.60, 0.70},
		{"single priority only", 1, 0, 10, 5, "context", 0.50, 0.60},
		{"multiple context", 0, 3, 0, 5, "context", 0.45, 0.65},
		{"single context", 0, 1, 0, 5, "context", 0.30, 0.40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel, conf := scoreRelevance(tt.priorityHits, tt.contextHits, tt.totalPriority, tt.totalContext)
			if rel != tt.wantRelevance {
				t.Errorf("relevance: got %q, want %q", rel, tt.wantRelevance)
			}
			if conf < tt.wantMinConf || conf > tt.wantMaxConf {
				t.Errorf("confidence: got %f, want [%f, %f]", conf, tt.wantMinConf, tt.wantMaxConf)
			}
		})
	}
}

func TestCountKeywordHits(t *testing.T) {
	text := "the cache uses lru eviction with connection pooling"
	keywords := []string{"cache", "lru", "pool", "memory", "allocation"}

	hits := countKeywordHits(text, keywords)
	// "cache", "lru", "pool" (substring of "pooling") = 3 hits
	if hits != 3 {
		t.Errorf("expected 3 hits, got %d", hits)
	}
}
