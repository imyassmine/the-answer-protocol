// Command gui is a small browser-based client for the TAP MUD server.
//
// It keeps a single TCP connection to the game server and exposes a web UI on
// http://localhost:8080. All shared state (socket, room, chat and logs) is
// guarded by appState.mu so HTTP handlers and the socket reader can run
// concurrently, and every page is rendered through html/template so values
// coming from the server are always HTML-escaped.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	listenAddr      = "127.0.0.1:8080"
	dialTimeout     = 5 * time.Second
	readHeaderLimit = 5 * time.Second
	// maxHistory bounds the chat and log panels so a long session cannot grow
	// memory without limit.
	maxHistory = 500
)

// roomView is an immutable snapshot of the last LOOK reply. It is replaced as
// a whole, never mutated, so snapshots can be shared with the renderer.
type roomView struct {
	Text        string
	Name        string
	Description string
	Players     []string
	Items       []string
	NPCs        []string
	Exits       []exitView
}

type exitView struct {
	Direction string
	Room      string
}

// pageData is everything the template needs, copied out of appState under the
// mutex before rendering.
type pageData struct {
	Connected bool
	Room      roomView
	Chat      string
	Logs      string
}

type appState struct {
	mu   sync.Mutex
	conn net.Conn
	room roomView
	chat []string
	logs []string
}

func newAppState() *appState {
	return &appState{
		room: roomView{Text: "Room info will appear here after LOOK."},
		chat: []string{"--- Chat Room ---"},
		logs: []string{"Not connected to the MUD server yet."},
	}
}

// snapshot returns a consistent copy of the state for one render pass.
func (s *appState) snapshot() pageData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return pageData{
		Connected: s.conn != nil,
		Room:      s.room,
		Chat:      strings.Join(s.chat, "\n"),
		Logs:      strings.Join(s.logs, "\n"),
	}
}

func appendCapped(lines []string, line string) []string {
	lines = append(lines, line)
	if len(lines) > maxHistory {
		lines = lines[len(lines)-maxHistory:]
	}
	return lines
}

func (s *appState) appendLogLocked(line string) {
	s.logs = appendCapped(s.logs, line)
}

func (s *appState) appendLog(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendLogLocked(line)
}

// connect dials addr, installs the new connection and starts the reader. On
// failure the previous connection (if any) is kept.
func (s *appState) connect(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		s.appendLog("[error] connection to " + addr + " failed: " + err.Error())
		return err
	}

	s.mu.Lock()
	old := s.conn
	s.conn = conn
	s.room = roomView{Text: "Room info will appear here after LOOK."}
	s.appendLogLocked("[connected] " + addr)
	s.mu.Unlock()

	if old != nil {
		_ = old.Close()
	}
	go s.readLoop(conn)
	return nil
}

func (s *appState) readLoop(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			s.handleLine(conn, strings.TrimSpace(line))
		}
		if err != nil {
			s.disconnected(conn, err)
			return
		}
	}
}

// disconnected clears the connection only if it is still the active one, so a
// reader left over from a previous connection cannot clobber a new one.
func (s *appState) disconnected(conn net.Conn, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != conn {
		return
	}
	s.conn = nil
	msg := "[disconnected] connection to the server was lost"
	if err != nil && !errors.Is(err, io.EOF) {
		msg += ": " + err.Error()
	}
	s.appendLogLocked(msg)
}

// handleLine dispatches one server line into the UI state. It drops the line
// when it comes from a connection that has since been replaced, so a stale
// reader cannot overwrite the room, chat or logs of the current session.
func (s *appState) handleLine(conn net.Conn, line string) {
	if line == "" {
		return
	}
	room, isRoom := parseRoomReply(line)
	chat := isChatEvent(line)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != conn {
		return
	}
	switch {
	case chat:
		s.chat = appendCapped(s.chat, line)
	case isRoom:
		s.room = room
	default:
		s.appendLogLocked("<< " + line)
	}
}

var chatEventPrefixes = []string{
	"EVT GLOBAL CHAT ",
	"EVT ROOM CHAT ",
	"EVT GROUP CHAT ",
}

// isChatEvent reports whether line is a chat broadcast emitted by the server.
func isChatEvent(line string) bool {
	for _, prefix := range chatEventPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

type lookReply struct {
	Room struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Exits       map[string]string `json:"exits"`
	} `json:"room"`
	Players []string `json:"players"`
	Items   []string `json:"items"`
	NPCs    []string `json:"npcs"`
}

// parseRoomReply turns an "OK {json}" LOOK response into a structured room
// view. Malformed or unrelated JSON is left to the log panel.
func parseRoomReply(line string) (roomView, bool) {
	payload, ok := strings.CutPrefix(line, "OK ")
	if !ok || !strings.HasPrefix(payload, "{") {
		return roomView{}, false
	}
	var reply lookReply
	if err := json.Unmarshal([]byte(payload), &reply); err != nil || reply.Room.Name == "" {
		return roomView{}, false
	}

	view := roomView{
		Text:        line,
		Name:        reply.Room.Name,
		Description: reply.Room.Description,
		Players:     reply.Players,
		Items:       reply.Items,
		NPCs:        reply.NPCs,
	}
	for direction, target := range reply.Room.Exits {
		view.Exits = append(view.Exits, exitView{Direction: direction, Room: target})
	}
	slices.SortFunc(view.Exits, func(a, b exitView) int {
		return strings.Compare(a.Direction, b.Direction)
	})
	return view, true
}

// send writes one protocol line to the server and records it in the log panel.
func (s *appState) send(cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		s.appendLog("[error] not connected to the server")
		return errors.New("not connected to the server")
	}

	if _, err := fmt.Fprintf(conn, "%s\n", cmd); err != nil {
		s.appendLog("[error] sending " + cmd + ": " + err.Error())
		return err
	}
	s.appendLog(">> " + cmd)
	return nil
}

var pageTemplate = template.Must(template.New("index").Funcs(template.FuncMap{
	"join": strings.Join,
}).Parse(pageHTML))

const pageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>TAP · Web Client</title>
	<style>
		body { font-family: monospace; background: #1e1e1e; color: #d4d4d4; padding: 20px; }
		h1 { font-size: 1.4em; font-weight: 600; margin: 0 0 8px; }
		h2 { font-size: 1em; font-weight: 600; margin: 18px 0 6px; color: #9cdcfe; }
		.box { background: #252526; border: 1px solid #333; padding: 10px; margin-bottom: 10px; height: 150px; overflow-y: auto; white-space: pre-wrap; }
		input, button { padding: 8px; margin: 4px; font-family: monospace; }
		input[type="text"] { background: #3c3c3c; color: #fff; border: 1px solid #555; width: 250px; }
		button { background: #007acc; color: white; border: none; cursor: pointer; }
		button:hover { background: #005999; }
		.panel { margin-bottom: 15px; }
		.status { margin: 0 0 14px; font-size: 0.9em; }
		.status.on { color: #4ec9b0; }
		.status.off { color: #f48771; }
	</style>
</head>
<body>
	<h1>TAP · Web Client</h1>
	<p class="status {{if .Connected}}on{{else}}off{{end}}">{{if .Connected}}● connected{{else}}○ disconnected{{end}}</p>

	<div class="panel">
		<form action="/connect" method="POST">
			IP: <input type="text" name="ip" value="127.0.0.1">
			Port: <input type="text" name="port" value="4242">
			Name: <input type="text" name="name" value="player">
			<button type="submit">Connect</button>
		</form>
	</div>

	<div class="panel">
		<strong>Quick Actions:</strong><br>
		<button onclick="sendCmd('LOOK')">LOOK</button>
		<button onclick="sendCmd('INVENTORY')">INVENTORY</button>
		<button onclick="sendCmd('WHO')">WHO</button>
		<button onclick="sendCmd('STATUS')">STATUS</button>
		<button onclick="sendCmd('QUESTS')">QUESTS</button>
		<button onclick="sendCmd('GROUP CREATE')">GROUP</button>
		<button onclick="sendCmd('QUIT')">QUIT</button>
		<br>
		<button onclick="sendCmd('MOVE north')">North</button>
		<button onclick="sendCmd('MOVE south')">South</button>
		<button onclick="sendCmd('MOVE east')">East</button>
		<button onclick="sendCmd('MOVE west')">West</button>
	</div>

	<div class="panel">
		Item: <input type="text" id="itemName" placeholder="item name">
		<button onclick="sendCustom('TAKE')">TAKE</button>
		<button onclick="sendCustom('DROP')">DROP</button>
		<br>
		NPC: <input type="text" id="npcName" placeholder="npc name">
		<button onclick="sendCustom('TALK')">TALK</button>
		<button onclick="sendCustom('ATTACK')">ATTACK</button>
	</div>

	<div class="panel">
		<form action="/send" method="POST">
			Custom Command: <input type="text" name="cmd" style="width: 400px;" placeholder="e.g. CHAT GLOBAL hello">
			<button type="submit">Send</button>
		</form>
	</div>

	<h2>Room &amp; Environment</h2>
	<div class="box" id="roomBox">{{if .Room.Name}}<strong>{{.Room.Name}}</strong>
{{.Room.Description}}{{with .Room.Exits}}
<strong>Exits:</strong> {{range .}}{{.Direction}} → {{.Room}}   {{end}}{{end}}{{with .Room.Players}}
<strong>Players:</strong> {{join . ", "}}{{end}}{{with .Room.Items}}
<strong>Items:</strong> {{join . ", "}}{{end}}{{with .Room.NPCs}}
<strong>NPCs:</strong> {{join . ", "}}{{end}}{{else}}{{.Room.Text}}{{end}}</div>

	<h2>Chat</h2>
	<div class="box" id="chatBox">{{.Chat}}</div>

	<h2>Server Logs &amp; Output</h2>
	<div class="box" id="logBox">{{.Logs}}</div>

	<script>
		function postCommand(action) {
			return fetch('/cmd', {
				method: 'POST',
				headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
				body: 'action=' + encodeURIComponent(action)
			}).then(() => location.reload());
		}
		function sendCmd(c) {
			postCommand(c);
		}
		function sendCustom(actionType) {
			let val = "";
			if (actionType === 'TAKE' || actionType === 'DROP') {
				val = document.getElementById('itemName').value;
			} else {
				val = document.getElementById('npcName').value;
			}
			postCommand(actionType + (val ? " " + val : ""));
		}
		// Keep the scrolling panels pinned to their latest line.
		window.onload = function() {
			['roomBox', 'chatBox', 'logBox'].forEach(id => {
				let el = document.getElementById(id);
				el.scrollTop = el.scrollHeight;
			});
		}
	</script>
</body>
</html>
`

func handleIndex(state *appState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var buf bytes.Buffer
		if err := pageTemplate.Execute(&buf, state.snapshot()); err != nil {
			log.Printf("render index: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(buf.Bytes()); err != nil {
			log.Printf("write index: %v", err)
		}
	}
}

func handleConnect(state *appState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		host := strings.TrimSpace(r.FormValue("ip"))
		port := strings.TrimSpace(r.FormValue("port"))
		if host == "" || port == "" {
			state.appendLog("[error] IP address and port are required to connect")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			name = "player"
		}
		if err := state.connect(net.JoinHostPort(host, port)); err == nil {
			_ = state.send("CONNECT " + name)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func handleSend(state *appState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		_ = state.send(r.FormValue("cmd"))
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func handleCmd(state *appState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		_ = state.send(r.FormValue("action"))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
	}
}

func main() {
	state := newAppState()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex(state))
	mux.HandleFunc("/connect", handleConnect(state))
	mux.HandleFunc("/send", handleSend(state))
	mux.HandleFunc("/cmd", handleCmd(state))

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderLimit,
	}
	log.Printf("TAP web client listening on http://%s", listenAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("web client stopped: %v", err)
	}
}
