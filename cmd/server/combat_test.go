package main

import (
	"strings"
	"testing"

	"github.com/imyassmine/tap/internal/world"
)

// testWorld builds a minimal world with one hostile goblin and one peaceful
// guard, both in the start room.
func testWorld(npcHP int) *world.World {
	return &world.World{
		Start: "loc.start",
		Rooms: map[string]world.Room{
			"loc.start": {Name: "Start", NPCs: []string{"npc.goblin", "npc.guard"}},
		},
		Items: map[string]world.Item{},
		NPCs: map[string]world.NPC{
			"npc.goblin": {Name: "Goblin", Dialogue: []string{"Grr"}, HP: npcHP, Hostile: true},
			"npc.guard":  {Name: "Village Guard", Dialogue: []string{"Stay safe."}, HP: 20},
		},
		Quests: map[string]world.Quest{},
	}
}

func newTestPlayer() *Player {
	return &Player{Name: "tester", Room: "loc.start", HP: playerMaxHP, MaxHP: playerMaxHP}
}

func TestStatus(t *testing.T) {
	s := NewServer(testWorld(30))
	p := newTestPlayer()

	if got, want := s.cmdStatus(p, ""), `OK {"hp":100,"max_hp":100,"status":"healthy"}`; got != want {
		t.Fatalf("cmdStatus() = %q, want %q", got, want)
	}
	if got := s.cmdStatus(p, "extra"); got != "ERR 400 BAD_REQUEST" {
		t.Fatalf("cmdStatus(extra) = %q, want ERR 400", got)
	}

	s.cmdDefend(p, "")
	if got := s.cmdStatus(p, ""); !strings.Contains(got, `"status":"defending"`) {
		t.Fatalf("cmdStatus() after DEFEND = %q, want defending", got)
	}

	p.HP = 40
	p.IsDefending = false
	if got := s.cmdStatus(p, ""); !strings.Contains(got, `"status":"wounded"`) {
		t.Fatalf("cmdStatus() at 40 HP = %q, want wounded", got)
	}
}

func TestAttackFlow(t *testing.T) {
	s := NewServer(testWorld(30))
	p := newTestPlayer()

	if got := s.cmdAttack(p, ""); got != "ERR 400 BAD_REQUEST" {
		t.Fatalf("cmdAttack() without target = %q", got)
	}
	if got := s.cmdAttack(p, "nobody"); got != "ERR 404 NPC_NOT_FOUND" {
		t.Fatalf("cmdAttack(nobody) = %q", got)
	}
	if got := s.cmdAttack(p, "Village Guard"); got != "ERR 405 NPC_NOT_HOSTILE" {
		t.Fatalf("cmdAttack(guard) = %q, want not hostile", got)
	}

	got := s.cmdAttack(p, "Goblin")
	want := `OK {"attacker_hp":90,"target_hp":15,"damage":15,"status":"combat"}`
	if got != want {
		t.Fatalf("first attack = %q, want %q", got, want)
	}

	got = s.cmdAttack(p, "Goblin")
	want = `OK {"attacker_hp":90,"target_hp":0,"damage":15,"status":"victory"}`
	if got != want {
		t.Fatalf("killing blow = %q, want %q", got, want)
	}

	if got := s.cmdAttack(p, "Goblin"); got != "ERR 404 NPC_NOT_FOUND" {
		t.Fatalf("attack dead NPC = %q, want not found", got)
	}
}

func TestDefendHalvesCounterAttack(t *testing.T) {
	s := NewServer(testWorld(30))
	p := newTestPlayer()

	s.cmdDefend(p, "")
	got := s.cmdAttack(p, "Goblin")
	want := `OK {"attacker_hp":95,"target_hp":15,"damage":15,"status":"combat"}`
	if got != want {
		t.Fatalf("attack after DEFEND = %q, want %q", got, want)
	}
	if p.IsDefending {
		t.Fatal("DEFEND state was not consumed by the counter-attack")
	}
}

func TestDefeatRespawnsPlayer(t *testing.T) {
	s := NewServer(testWorld(500))
	p := newTestPlayer()

	var last string
	for i := 0; i < 10; i++ {
		last = s.cmdAttack(p, "Goblin")
	}
	if !strings.Contains(last, `"status":"defeated"`) {
		t.Fatalf("tenth attack = %q, want defeated", last)
	}
	if p.HP != respawnHP {
		t.Errorf("HP after respawn = %d, want %d", p.HP, respawnHP)
	}
	if p.Room != "loc.start" {
		t.Errorf("room after respawn = %q, want loc.start", p.Room)
	}
}
