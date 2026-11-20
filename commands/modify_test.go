package commands_test

import (
	"strings"
	"testing"

	"github.com/golem-mc/multiverse/internal/mvtest"
)

func TestModifySetsAddsRemovesAndResets(t *testing.T) {
	env := mvtest.Start(t, nil)
	env.Expect(env.Console("mv modify world set gamemode creative"),
		"Successfully set 'gamemode' to 'creative' in world world.")
	env.Expect(env.Console("mv modify world set entryfee-amount 5"),
		"Successfully set 'entryfee-amount' to '5.0' in world world.")
	env.Expect(env.Console("mv modify world add world-blacklist world_nether"),
		"Successfully add 'world-blacklist' to '[world_nether]' in world world.")
	if saved := env.ReadFile("worlds.yml"); !strings.Contains(saved, "gamemode: creative") ||
		!strings.Contains(saved, "world-blacklist:\n    - world_nether") {
		t.Errorf("worlds.yml lacks the changes:\n%s", saved)
	}
	out := env.Console("mv modify world remove world-blacklist world_the_end")
	env.Expect(out, "Failed to remove 'world-blacklist' to 'world_the_end' in world world.")
	env.Expect(out, "Cannot remove item as it is already not in the list!")
	env.Expect(env.Console("mv modify world remove world-blacklist world_nether"),
		"Successfully remove 'world-blacklist' to '[]' in world world.")
	env.Expect(env.Console("mv modify world reset gamemode"),
		"Successfully reset 'gamemode' to 'survival' in world world.")

	out = env.Console("mv modify world set gamemode flying")
	env.Expect(out, "Failed to set 'gamemode' to 'flying' in world world.")
	env.Expect(out, "No enum constant FLYING")
	env.Expect(env.Console("mv modify world add gamemode creative"), "Node not found in config: gamemode")
	env.Expect(env.Console("mv modify world reset nope"), "Failed to reset 'nope' in world world.")
	env.Expect(env.Console("mv modify world reset gamemode creative"), "You cannot specify a value to reset 'gamemode'.")
	env.Expect(env.Console("mv modify world set gamemode"), "You must specify a value to set 'gamemode'.")
	env.Expect(env.Console("mv modify world set animals false"),
		"You cannot modify 'animals' because it has been changed or removed. Please use '/mv entity-spawn-config modify [world] animal set spawn <true|false>' instead.")
	out = env.Console("mv modify world toggle pvp")
	env.Expect(out, "Please specify one of (set, add, remove, reset).")
	env.Expect(out, "Usage: /mv modify [world] <set|add|remove|reset> <property> [value]")
	env.Expect(env.Console("mv modify set pvp false"), "World 'set' is not a multiverse world. Remember to specify the world name")
	env.Expect(env.Console("mv modify nowhere set pvp false"), "World 'nowhere' is not a multiverse world.")

	out = env.Console("mv modify world_nether set alias World")
	env.Expect(out, "Successfully set 'alias' to 'World' in world world_nether.")
	env.Expect(out, "Alias conflict detected for world 'world_nether' with alias 'World'.")
	env.Expect(out, "- Conflicts with world name: 'world'")
}

func TestModifyCompletesAndDefaultsToThePlayersWorld(t *testing.T) {
	env := mvtest.Start(t, nil, plugintestOps("Alice"))
	console := env.MV.Server.Console()
	for line, want := range map[string]string{
		"modify world_n":                         "modify world_nether",
		"modify world ":                          "modify world add,modify world remove,modify world reset,modify world set",
		"modify world add ":                      "modify world add world-blacklist",
		"modify world set game":                  "modify world set gamemode",
		"modify world set gamemode ad":           "modify world set gamemode adventure",
		"modify world set respawn-world world_t": "modify world set respawn-world world_the_end",
	} {
		if got := strings.Join(env.MV.Commands.Suggest(console, nil, line), ","); got != want {
			t.Errorf("Suggest(%q) = %q, want %q", line, got, want)
		}
	}
	alice := env.Join("Alice", true)
	if got := strings.Join(env.MV.Commands.Suggest(alice.Handle(), nil, "modify set pv"), ","); got != "modify set pvp" {
		t.Errorf("player completion = %q", got)
	}
	env.Expect(mvtest.Run(alice, "mv modify set pvp false"), "Successfully set 'pvp' to 'false' in world world.")
	env.Expect(mvtest.Run(alice, "mv modify world_nether set pvp false"), "Successfully set 'pvp' to 'false' in world world_nether.")
	if w, _ := env.MV.Worlds.Get("world"); w.Props.Bool("pvp") {
		t.Error("pvp is still on in world")
	}
}
