package main

import "testing"

func TestCleanText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trims spaces", "  hello  ", "hello"},
		{"removes control characters", "ali\x01ce", "alice"},
		{"removes tabs and newlines", "a\tb\nc", "abc"},
		{"keeps unicode", "héllo wörld", "héllo wörld"},
		{"keeps inner spaces", "Village Guard", "Village Guard"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanText(tt.input); got != tt.want {
				t.Errorf("cleanText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchRef(t *testing.T) {
	tests := []struct {
		name  string
		query string
		id    string
		ref   string
		want  bool
	}{
		{"matches id", "npc.goblin", "npc.goblin", "Goblin", true},
		{"matches name case-insensitively", "MASTER BAKER", "npc.baker", "Master Baker", true},
		{"does not match a name fragment", "baker", "npc.baker", "Master Baker", false},
		{"does not match another ref", "guard", "npc.goblin", "Goblin", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchRef(tt.query, tt.id, tt.ref); got != tt.want {
				t.Errorf("matchRef(%q, %q, %q) = %v, want %v", tt.query, tt.id, tt.ref, got, tt.want)
			}
		})
	}
}
