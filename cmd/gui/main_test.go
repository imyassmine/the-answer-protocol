package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestParseRoomReply(t *testing.T) {
	line := `OK {"room":{"id":"loc.square","name":"Village Square","description":"A bustling square.","exits":{"north":"loc.tavern","east":"loc.shop"}},"players":["alice"],"items":["item.ale"],"npcs":["npc.guard"]}`
	view, ok := parseRoomReply(line)
	if !ok {
		t.Fatalf("parseRoomReply(%q) = _, false", line)
	}
	if view.Name != "Village Square" {
		t.Errorf("Name = %q, want Village Square", view.Name)
	}
	if view.Description != "A bustling square." {
		t.Errorf("Description = %q, want A bustling square.", view.Description)
	}
	if len(view.Players) != 1 || view.Players[0] != "alice" {
		t.Errorf("Players = %v, want [alice]", view.Players)
	}
	if len(view.Items) != 1 || view.Items[0] != "item.ale" {
		t.Errorf("Items = %v, want [item.ale]", view.Items)
	}
	if len(view.NPCs) != 1 || view.NPCs[0] != "npc.guard" {
		t.Errorf("NPCs = %v, want [npc.guard]", view.NPCs)
	}
	if len(view.Exits) != 2 || view.Exits[0].Direction != "east" || view.Exits[1].Direction != "north" {
		t.Errorf("Exits = %v, want sorted east, north", view.Exits)
	}

	for _, line := range []string{"ERR 400 BAD_REQUEST", "OK not-json", ""} {
		if _, ok := parseRoomReply(line); ok {
			t.Errorf("parseRoomReply(%q) = _, true, want false", line)
		}
	}
}

func TestIsChatEvent(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"EVT GLOBAL CHAT alice hi", true},
		{"EVT ROOM CHAT bob hello", true},
		{"EVT GROUP CHAT team hi", true},
		{"EVT ROOM PRESENCE ENTER bob", false},
		{"OK connected", false},
	}
	for _, tt := range tests {
		if got := isChatEvent(tt.line); got != tt.want {
			t.Errorf("isChatEvent(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestAppendCapped(t *testing.T) {
	var lines []string
	for i := 0; i < maxHistory+10; i++ {
		lines = appendCapped(lines, fmt.Sprintf("%d", i))
	}
	if len(lines) != maxHistory {
		t.Fatalf("len(lines) = %d, want %d", len(lines), maxHistory)
	}
	if lines[0] != "10" {
		t.Errorf("first line = %q, want 10", lines[0])
	}
	if last := lines[len(lines)-1]; last != fmt.Sprintf("%d", maxHistory+9) {
		t.Errorf("last line = %q, want %d", last, maxHistory+9)
	}
}

func TestIndexEscapesServerText(t *testing.T) {
	state := newAppState()
	state.mu.Lock()
	state.room = roomView{
		Name:        "<script>alert(1)</script>",
		Description: "<b>html</b>",
	}
	state.mu.Unlock()

	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, state.snapshot()); err != nil {
		t.Fatalf("template execution error = %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>alert(1)</script>") || strings.Contains(out, "<b>html</b>") {
		t.Fatal("server text was rendered unescaped")
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("expected HTML-escaped room name")
	}
}
