package registry

import (
	"testing"
)

func TestSkillFormatInstalls(t *testing.T) {
	tests := []struct {
		installs int
		want     string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{12345, "12.3K"},
		{123897, "123.9K"},
		{999999, "1000.0K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{10000000, "10.0M"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			s := Skill{Installs: tc.installs}
			got := s.FormatInstalls()
			if got != tc.want {
				t.Errorf("FormatInstalls() for %d = %q, want %q", tc.installs, got, tc.want)
			}
		})
	}
}

func TestSkillTreeURL(t *testing.T) {
	s := Skill{Source: "anthropics/skills", SkillID: "frontend-design"}
	got := s.TreeURL("main")
	want := "https://github.com/anthropics/skills/tree/main/skills/frontend-design"
	if got != want {
		t.Errorf("TreeURL() = %q, want %q", got, want)
	}

	got = s.TreeURL("master")
	want = "https://github.com/anthropics/skills/tree/master/skills/frontend-design"
	if got != want {
		t.Errorf("TreeURL(master) = %q, want %q", got, want)
	}
}

func TestFormatCount(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{500, "500"},
		{1000, "1.0K"},
		{85747, "85.7K"},
		{1000000, "1.0M"},
	}
	for _, tc := range tests {
		got := FormatCount(tc.n)
		if got != tc.want {
			t.Errorf("FormatCount(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestParseSearchResponse(t *testing.T) {
	body := []byte(`{
		"query": "python",
		"searchType": "fuzzy",
		"skills": [
			{"id": "wshobson/agents/python-perf", "skillId": "python-perf", "name": "python-perf", "installs": 7485, "source": "wshobson/agents"},
			{"id": "org/repo/python-test", "skillId": "python-test", "name": "python-test", "installs": 6123, "source": "org/repo"}
		],
		"count": 2,
		"duration_ms": 24
	}`)

	skills, err := ParseSearchResponse(body)
	if err != nil {
		t.Fatalf("ParseSearchResponse() error: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	if skills[0].Name != "python-perf" {
		t.Errorf("skills[0].Name = %q, want %q", skills[0].Name, "python-perf")
	}
	if skills[0].Source != "wshobson/agents" {
		t.Errorf("skills[0].Source = %q, want %q", skills[0].Source, "wshobson/agents")
	}
	if skills[0].Installs != 7485 {
		t.Errorf("skills[0].Installs = %d, want 7485", skills[0].Installs)
	}
	if skills[1].SkillID != "python-test" {
		t.Errorf("skills[1].SkillID = %q, want %q", skills[1].SkillID, "python-test")
	}
}

func TestParseSearchResponseError(t *testing.T) {
	body := []byte(`{"error": "Query must be at least 2 characters"}`)
	_, err := ParseSearchResponse(body)
	if err == nil {
		t.Error("expected error for API error response, got nil")
	}
}

func TestParseSearchResponseEmpty(t *testing.T) {
	body := []byte(`{"query": "zzzzz", "searchType": "fuzzy", "skills": [], "count": 0}`)
	skills, err := ParseSearchResponse(body)
	if err != nil {
		t.Fatalf("ParseSearchResponse() error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestParseSearchResponseInvalidJSON(t *testing.T) {
	_, err := ParseSearchResponse([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestSearch(t *testing.T) {
	skills := []Skill{
		{Source: "anthropics/skills", SkillID: "frontend-design", Name: "frontend-design", Installs: 100},
		{Source: "vercel-labs/agent-skills", SkillID: "react-best-practices", Name: "react-best-practices", Installs: 200},
		{Source: "microsoft/copilot", SkillID: "azure-deploy", Name: "azure-deploy", Installs: 300},
		{Source: "some-org/python-tools", SkillID: "python-linting", Name: "python-linting", Installs: 50},
	}

	tests := []struct {
		query string
		want  int
	}{
		{"", 4},             // empty query returns all
		{"frontend", 1},     // matches name
		{"react", 1},        // matches name
		{"azure", 1},        // matches name
		{"python", 1},       // matches name and source
		{"anthropics", 1},   // matches source only
		{"vercel", 1},       // matches source only
		{"microsoft", 1},    // matches source only
		{"nonexistent", 0},  // no matches
		{"FRONTEND", 1},     // case insensitive
		{"deploy", 1},       // partial match
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			results := Search(skills, tc.query)
			if len(results) != tc.want {
				t.Errorf("Search(%q) returned %d results, want %d", tc.query, len(results), tc.want)
			}
		})
	}
}
