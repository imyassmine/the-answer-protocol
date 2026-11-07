package main

import (
	"encoding/json"
	"log/slog"
	"slices"
)

func (s *Server) cmdQuests(args string) string {
	if args != "" {
		return "ERR 400 BAD_REQUEST"
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()

	questIDs := make([]string, 0, len(s.world.Quests))
	for id := range s.world.Quests {
		questIDs = append(questIDs, id)
	}
	slices.Sort(questIDs)

	data, err := json.Marshal(questIDs)
	if err != nil {
		slog.Error("quests marshal failed", "error", err)
		return "ERR 500 INTERNAL_ERROR"
	}
	return "OK " + string(data)
}

func (s *Server) cmdQuest(p *Player, args string) string {
	if args == "" {
		return "ERR 400 BAD_REQUEST"
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()

	quest, ok := s.world.Quests[args]
	if !ok {
		return "ERR 404 QUEST_NOT_FOUND"
	}

	data, err := json.Marshal(quest)
	if err != nil {
		slog.Error("quest marshal failed", "player", p.Name, "error", err)
		return "ERR 500 INTERNAL_ERROR"
	}
	return "OK " + string(data)
}
