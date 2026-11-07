package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type World struct {
	Start  string           `json:"start"`
	Rooms  map[string]Room  `json:"rooms"`
	Items  map[string]Item  `json:"items"`
	NPCs   map[string]NPC   `json:"npcs"`
	Quests map[string]Quest `json:"quests"`
}

type Room struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Exits       map[string]string `json:"exits"`
	Items       []string          `json:"items"`
	NPCs        []string          `json:"npcs"`
}

type Item struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Obtainable  bool   `json:"obtainable"`
}

type NPC struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Dialogue    []string `json:"dialogue"`
	HP          int      `json:"hp"`
	Hostile     bool     `json:"hostile"`
}

type Quest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Target      string `json:"target"`
	Reward      string `json:"reward"`
}

func Load(path string) (*World, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var w World
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&w); err != nil {
		return nil, err
	}
	if err := w.Validate(); err != nil {
		return nil, err
	}
	return &w, nil
}

func (w *World) Validate() error {
	var errs []error
	if _, ok := w.Rooms[w.Start]; !ok {
		errs = append(errs, fmt.Errorf("start room %q does not exist", w.Start))
	}
	for roomID, room := range w.Rooms {
		for direction, target := range room.Exits {
			if _, ok := w.Rooms[target]; !ok {
				errs = append(errs, fmt.Errorf("room %q: exit %q leads to unknown room %q", roomID, direction, target))
			}
		}
		for _, itemID := range room.Items {
			if _, ok := w.Items[itemID]; !ok {
				errs = append(errs, fmt.Errorf("room %q: unknown item %q", roomID, itemID))
			}
		}
		for _, npcID := range room.NPCs {
			if _, ok := w.NPCs[npcID]; !ok {
				errs = append(errs, fmt.Errorf("room %q: unknown NPC %q", roomID, npcID))
			}
		}
	}
	for npcID, npc := range w.NPCs {
		if len(npc.Dialogue) == 0 {
			errs = append(errs, fmt.Errorf("NPC %q has no dialogue", npcID))
		}
	}
	for questID, quest := range w.Quests {
		_, isItem := w.Items[quest.Target]
		_, isNPC := w.NPCs[quest.Target]
		if !isItem && !isNPC {
			errs = append(errs, fmt.Errorf("quest %q: unknown target %q", questID, quest.Target))
		}
		if quest.Reward != "" {
			if _, ok := w.Items[quest.Reward]; !ok {
				errs = append(errs, fmt.Errorf("quest %q: unknown reward %q", questID, quest.Reward))
			}
		}
	}
	return errors.Join(errs...)
}
