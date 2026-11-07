package world

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRepositoryWorld(t *testing.T) {
	w, err := Load(filepath.Join("..", "..", "data", "world.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if w.Start != "loc.square" {
		t.Errorf("Start = %q, want loc.square", w.Start)
	}
	if got, want := len(w.Rooms), 8; got != want {
		t.Errorf("len(Rooms) = %d, want %d", got, want)
	}
	if got, want := len(w.Items), 4; got != want {
		t.Errorf("len(Items) = %d, want %d", got, want)
	}
	if got, want := len(w.NPCs), 4; got != want {
		t.Errorf("len(NPCs) = %d, want %d", got, want)
	}
	if got, want := len(w.Quests), 2; got != want {
		t.Errorf("len(Quests) = %d, want %d", got, want)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "world.json")
	data := `{"start":"room.a","rooms":{},"items":{},"npcs":{},"quests":{},"bogus":true}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() accepted an unknown field")
	} else if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Load() error = %v, want an unknown-field error", err)
	}
}

func TestValidate(t *testing.T) {
	validWorld := func() *World {
		return &World{
			Start: "room.a",
			Rooms: map[string]Room{
				"room.a": {
					Name:        "Room A",
					Description: "A room.",
					Exits:       map[string]string{"north": "room.b"},
					Items:       []string{"item.x"},
					NPCs:        []string{"npc.x"},
				},
				"room.b": {Name: "Room B"},
			},
			Items: map[string]Item{"item.x": {Name: "Item X", Obtainable: true}},
			NPCs: map[string]NPC{"npc.x": {
				Name:     "NPC X",
				Dialogue: []string{"hello"},
				HP:       10,
			}},
			Quests: map[string]Quest{"quest.x": {
				Title:  "Quest X",
				Target: "item.x",
				Reward: "item.x",
			}},
		}
	}

	if err := validWorld().Validate(); err != nil {
		t.Fatalf("Validate() on a valid world = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*World)
		want   string
	}{
		{"unknown start room", func(w *World) { w.Start = "room.missing" }, "start room"},
		{"unknown exit target", func(w *World) {
			w.Rooms["room.a"].Exits["south"] = "room.missing"
		}, "unknown room"},
		{"unknown room item", func(w *World) {
			room := w.Rooms["room.a"]
			room.Items = append(room.Items, "item.missing")
			w.Rooms["room.a"] = room
		}, "unknown item"},
		{"unknown room npc", func(w *World) {
			room := w.Rooms["room.a"]
			room.NPCs = append(room.NPCs, "npc.missing")
			w.Rooms["room.a"] = room
		}, "unknown NPC"},
		{"npc without dialogue", func(w *World) {
			w.NPCs["npc.mute"] = NPC{Name: "Mute"}
		}, "has no dialogue"},
		{"unknown quest target", func(w *World) {
			q := w.Quests["quest.x"]
			q.Target = "item.missing"
			w.Quests["quest.x"] = q
		}, "unknown target"},
		{"unknown quest reward", func(w *World) {
			q := w.Quests["quest.x"]
			q.Reward = "item.missing"
			w.Quests["quest.x"] = q
		}, "unknown reward"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := validWorld()
			tt.mutate(w)
			err := w.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}
