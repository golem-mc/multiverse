package commands_test

import (
	"strings"
	"testing"

	_ "github.com/golem-mc/multiverse"
	"github.com/golem-mc/multiverse/core"
	"github.com/golem-mc/multiverse/internal/mvtest"
)

// A fresh server imports the three vanilla worlds into worlds.yml, under
// their keys, and writes config.yml with every option.
func TestFreshServerImportsDefaultWorlds(t *testing.T) {
	env := mvtest.Start(t, nil)
	worlds := env.ReadFile("worlds.yml")
	for _, want := range []string{"minecraft:overworld:", "minecraft:the_nether:", "minecraft:the_end:",
		"legacy-world-name: world_nether", "environment: the_end", "scale: 8.0", "==: MVSpawnLocation",
		"tick-rate: '@unset'", "currency: '@vault-economy'", "version: 1.3"} {
		if !strings.Contains(worlds, want) {
			t.Errorf("worlds.yml lacks %q:\n%s", want, worlds)
		}
	}
	cfg := env.ReadFile("config.yml")
	for _, want := range []string{"\n  auto-import-default-worlds: true", "  confirm-mode: enable",
		"    nether: '%overworld%_nether'", "# When enabled, Multiverse will automatically import", "version: 5.2"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config.yml lacks %q:\n%s", want, cfg)
		}
	}
	out := env.Console("mv list")
	env.Expect(out, "====[ Multiverse World List ]====")
	env.Expect(out, "world - NORMAL")
	env.Expect(out, "world_nether - NETHER")
	env.Expect(out, "world_the_end - THE_END")
}

// An admin's worlds.yml, keyed by name, loads its worlds with their
// generator and keeps their properties; hidden worlds and unloaded ones show
// as /mv list shows them.
func TestWorldsYmlLoadsWorlds(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"worlds.yml": `
creative:
  alias: '&bCreative'
  gamemode: creative
  generator: Void
  hidden: true
  read-only:
    environment: normal
    seed: 7
lobby:
  auto-load: false
  generator: Floor
  read-only:
    environment: normal
`})
	w, ok := env.MV.Worlds.Get("creative")
	if !ok || !w.Loaded() {
		t.Fatalf("creative not loaded: %v", ok)
	}
	if w.Key().String() != "minecraft:creative" || w.ColourlessAlias() != "Creative" {
		t.Errorf("key %s alias %q", w.Key(), w.ColourlessAlias())
	}
	if h, _ := w.Handle(); h.Seed() != 7 {
		t.Errorf("seed = %d", h.Seed())
	}
	if lobby, ok := env.MV.Worlds.Get("lobby"); !ok || lobby.Loaded() {
		t.Error("lobby should be known and unloaded")
	}
	out := env.Console("mv list")
	env.Expect(out, "[H] Creative - NORMAL")
	env.Expect(out, "lobby - UNLOADED")
	out = env.Console("mv list --raw --filter creat")
	env.Expect(out, "[Filter 'creat']")
	env.Expect(out, "creative - NORMAL")
	env.ExpectNot(out, "lobby")

	saved := env.ReadFile("worlds.yml")
	if !strings.Contains(saved, "creative:\n  adjust-spawn: false\n  alias: '&bCreative'") || !strings.Contains(saved, "gamemode: creative") {
		t.Errorf("worlds.yml lost creative's entry:\n%s", saved)
	}
}

func TestInfoShowsProperties(t *testing.T) {
	env := mvtest.Start(t, nil)
	out := env.Console("mv info world_nether")
	env.Expect(out, "---- World Info: world_nether ----")
	env.Expect(out, "World Key: minecraft:the_nether")
	env.Expect(out, "Environment: NETHER")
	env.Expect(out, "World Scale: 8.0")
	env.Expect(out, "Entry Fee: FREE!")
	env.Expect(out, "World Alias: empty")
	out = env.Console("mv info")
	env.Expect(out, "This command can only be used by a player in a loaded Multiverse World.")
	out = env.Console("mv info nowhere")
	env.Expect(out, "World 'nowhere' is not a loaded multiverse world.")
	env.Expect(out, "Usage: /mv info [world]")
}

func TestPlayerInfoPagesAndDefaultsToTheirWorld(t *testing.T) {
	env := mvtest.Start(t, nil, plugintestOps("Alice"))
	alice := env.Join("Alice", true)
	out := mvtest.Run(alice, "mv info --page 2")
	env.Expect(out, "---- World Info: world ----")
	env.Expect(out, "[Page 2 of 4]")
	env.Expect(out, "Bed Respawn: true")
	out = mvtest.Run(alice, "mv info world --page 9")
	env.Expect(out, "Invalid page number. Please enter a page number between 1 and 4.")
}

func TestConfigCommand(t *testing.T) {
	env := mvtest.Start(t, nil)
	env.Expect(env.Console("mv config enforce-access"), "Config 'enforce-access' is currently set to 'false'.")
	env.Expect(env.Console("mv config enforce-access true"), "Successfully set config 'enforce-access' to 'true'.")
	if !env.MV.Config.Bool("world.enforce-access") || !strings.Contains(env.ReadFile("config.yml"), "enforce-access: true") {
		t.Error("enforce-access was not set and saved")
	}
	out := env.Console("mv config enforce-access maybe")
	env.Expect(out, "Failed to set config 'enforce-access' to 'maybe'.")
	env.Expect(out, "Unable to convert 'maybe' to boolean. Please use 'true' or 'false'")
	env.Expect(env.Console("mv config confirm-timeout 0"), "Confirm timeout must be a positive number!")
	env.Expect(env.Console("mv config nope"), "Node not found in config: nope")
	env.Expect(env.Console("mv debug 2"), "Multiverse Debug mode is at level 2.")
	env.Expect(env.Console("mv debug"), "Multiverse Debug mode is at level 2.")
	env.Expect(env.Console("mv debug 0"), "Multiverse Debug mode is OFF.")
	env.Expect(env.Console("mv debug 7"), "Debug level must be between 0 and 3.")
}

func TestRootHelpVersionReload(t *testing.T) {
	env := mvtest.Start(t, nil)
	out := env.Console("mv")
	env.Expect(out, "Multiverse-Core version "+core.Version)
	env.Expect(out, "See /mv help for commands available.")
	env.Expect(env.Console("mv version"), "Special Code: FRN002")
	env.Expect(env.Console("mv help"), "Showing help for /mv")
	out = env.Console("mv help list")
	env.Expect(out, "/mv list [--filter <filter>] [--page <page>] [--raw] - Displays a listing of all worlds that you can enter.")
	env.Expect(env.Console("mv bogus"), "Showing help for /mv")
	out = env.Console("mv reload")
	env.Expect(out, "Reloading all Multiverse Plugin configs...")
	env.Expect(out, "Multiverse-Core - worlds.yml")
	env.Expect(out, "Reload complete!")
	env.Expect(env.Console("mv confirm"), "You do not have any commands in queue.")
}

func TestCompletionAndPermissions(t *testing.T) {
	env := mvtest.Start(t, nil)
	console := env.MV.Server.Console()
	got := env.MV.Commands.Suggest(console, nil, "inf")
	if strings.Join(got, ",") != "info" {
		t.Errorf("subcommand completion = %v", got)
	}
	got = env.MV.Commands.Suggest(console, nil, "info world_")
	if strings.Join(got, ",") != "info world_nether,info world_the_end" {
		t.Errorf("world completion = %v", got)
	}
	got = env.MV.Commands.Suggest(console, nil, "list --")
	if strings.Join(got, ",") != "list --filter,list --page,list --raw" {
		t.Errorf("flag completion = %v", got)
	}
	got = env.MV.Commands.Suggest(console, nil, "config confirm-mode ")
	if len(got) != 5 || got[0] != "config confirm-mode disable" {
		t.Errorf("config value completion = %v", got)
	}

	bob := env.Join("Bob", false)
	env.Expect(mvtest.Run(bob, "mv list"), "I'm sorry, but you do not have permission to perform this command.")
	var none []string
	env.Sync(func() {
		p, _ := env.MV.Server.PlayerNamed("Bob")
		none = env.MV.Commands.Suggest(p, nil, "")
	})
	if len(none) != 0 {
		t.Errorf("a player without permissions is offered %v", none)
	}
}

// show-legacy-aliases registers Multiverse 4's root commands.
func TestLegacyAliases(t *testing.T) {
	env := mvtest.Start(t, map[string]string{"config.yml": "command:\n  show-legacy-aliases: true\n"})
	env.Expect(env.Console("mvl"), "world - NORMAL")
	env.Expect(env.Console("mvinfo world_the_end"), "Environment: THE_END")
	env.Expect(env.Console("mvconf use-confirm-otp"), "Config 'use-confirm-otp' is currently set to 'true'.")
}
