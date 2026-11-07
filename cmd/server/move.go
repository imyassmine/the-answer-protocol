package main

import "log/slog"

func (s *Server) cmdMove(p *Player, args string) string {
	if args == "" {
		return "ERR 400 BAD_REQUEST"
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()
	from := p.Room
	target, ok := s.world.Rooms[from].Exits[args]
	if !ok {
		return "ERR 301 NO_EXIT"
	}
	p.Room = target
	s.notifyRoomChangeLocked(from, target, p.Name)
	slog.Info("player moved", "player", p.Name, "from", from, "to", target)
	return "OK room=" + target
}
