package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golem-mc/multiverse/core"
	"github.com/golem-mc/multiverse/internal/mvtest"
)

// A Multiverse 4 worlds.yml is kept as worlds.yml.old.<millis> and
// rewritten in version 5's form, its worlds loaded with their old settings.
func TestMV4WorldsYmlMigrates(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"worlds.yml": `worlds:
  lobby:
    ==: MVWorld
    alias: Lobby
    color: AQUA
    style: NORMAL
    environment: NORMAL
    generator: Void
    gameMode: CREATIVE
    pvp: 'false'
    autoLoad: 'true'
    seed: '42'
    spawning:
      ==: MVSpawnSettings
      monsters:
        ==: MVSpawnSubSettings
        spawn: 'false'
        spawnrate: '-1'
        exceptions: []
  old:
    world:
      ==: MVWorld
      environment: NORMAL
      autoLoad: 'false'
`})
	lobby := world(env, "lobby")
	if h, ok := lobby.Handle(); !ok || h.Seed() != 42 {
		t.Fatal("lobby is not loaded with seed 42")
	}
	if lobby.Alias() != "&bLobby" || lobby.Props.String(core.PropGameMode) != "creative" || lobby.Props.Bool(core.PropPvp) {
		t.Errorf("lobby: alias %q gamemode %q pvp %v", lobby.Alias(), lobby.Props.String(core.PropGameMode), lobby.Props.Bool(core.PropPvp))
	}
	if lobby.Spawning().Allows("monster", "zombie") || lobby.Spawning().Category("monster").Int("tick-rate") != core.SpawnServer {
		t.Error("lobby's monster spawning was not migrated")
	}
	if old := world(env, "old.world"); old.Loaded() {
		t.Error("old.world should stay unloaded")
	}
	saved := env.ReadFile("worlds.yml")
	for _, want := range []string{"\nlobby:\n", "\nold[dot]world:\n", "  version: 1.3\n", "    monster:\n", "legacy-world-name: lobby"} {
		if !strings.Contains("\n"+saved, want) {
			t.Errorf("worlds.yml lacks %q:\n%s", want, saved)
		}
	}
	if strings.Contains(saved, "worlds:") || strings.Contains(saved, "gameMode") {
		t.Errorf("worlds.yml keeps Multiverse 4's form:\n%s", saved)
	}
	backups, _ := filepath.Glob(filepath.Join(env.DataDir, "worlds.yml.old.*"))
	if len(backups) != 1 {
		t.Fatalf("backups = %v", backups)
	}
	if data, _ := os.ReadFile(backups[0]); !strings.Contains(string(data), "==: MVWorld") {
		t.Errorf("the backup is not the old file:\n%s", data)
	}
	env.Expect(env.Console("mv list"), "Lobby - NORMAL")
}
