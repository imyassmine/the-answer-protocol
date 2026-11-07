package main

import (
	"log/slog"
	"strings"
)

func (s *Server) cmdChat(p *Player, args string) string {
	scope, text, _ := strings.Cut(args, " ")
	text = cleanText(text)
	if text == "" {
		return "ERR 400 BAD_REQUEST"
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()
	evt := "EVT " + scope + " CHAT " + p.Name + " " + text
	switch scope {
	case "GLOBAL":
		s.pushAllLocked(evt)
	case "ROOM":
		s.pushRoomLocked(p.Room, "", evt)
	case "GROUP":
		if p.Group == nil {
			return "ERR 401 NOT_IN_GROUP"
		}
		s.pushGroupLocked(p.Group, evt)
	default:
		return "ERR 400 BAD_REQUEST"
	}
	return "OK"
}

func (s *Server) pushAllLocked(msg string) {
	for _, p := range s.players {
		p.push(msg)
	}
}

func (s *Server) pushRoomLocked(roomID, except, msg string) {
	for _, p := range s.players {
		if p.Room == roomID && p.Name != except {
			p.push(msg)
		}
	}
}

// notifyRoomChangeLocked announces a player leaving one room and entering another.
func (s *Server) notifyRoomChangeLocked(from, to, name string) {
	s.pushRoomLocked(from, name, "EVT ROOM PRESENCE LEAVE "+name)
	s.pushRoomLocked(to, name, "EVT ROOM PRESENCE ENTER "+name)
}

func (player *Player) push(msg string) {
	select {
	case player.out <- msg:
	default:
		slog.Warn("message dropped", "player", player.Name)
	}
}
