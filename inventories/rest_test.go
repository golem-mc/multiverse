package inventories_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/vault"

	"github.com/golem-mc/multiverse/internal/mvtest"
)

// bank is a vault.Economy with balances in memory.
type bank struct {
	mu       sync.Mutex
	balances map[plugin.UUID]float64
}

func (*bank) Name() string                                       { return "Test" }
func (*bank) FractionalDigits() int                              { return 2 }
func (*bank) Format(a float64) string                            { return fmt.Sprintf("$%.2f", a) }
func (*bank) CurrencyNameSingular() string                       { return "dollar" }
func (*bank) CurrencyNamePlural() string                         { return "dollars" }
func (*bank) HasAccount(plugin.UUID) bool                        { return true }
func (*bank) CreateAccount(plugin.UUID, string) bool             { return true }
func (b *bank) Has(id plugin.UUID, a float64) bool               { return b.Balance(id) >= a }
func (b *bank) Deposit(id plugin.UUID, a float64) vault.Response { return b.Withdraw(id, -a) }

func (b *bank) Balance(id plugin.UUID) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.balances[id]
}

func (b *bank) Withdraw(id plugin.UUID, a float64) vault.Response {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.balances[id] -= a
	return vault.Response{Amount: a, Balance: b.balances[id]}
}

// Statistics and, through vault.Economy, the balance are switched per
// world and saved under Bukkit's statistic names.
func TestStatisticsAndEconomyAreSwitched(t *testing.T) {
	env, _ := start(t, map[string]string{inv + "config.yml": `share-handling:
  active-optional-shares:
  - economy
first-run: false
`})
	createWorlds(env, "other")
	b := &bank{balances: map[plugin.UUID]float64{}}
	vault.ProvideEconomy(env.MV.Server, b, vault.PriorityNormal)
	p := env.Join("Stat", true)
	id := p.Handle().UUID()
	jump := plugin.Statistic{Type: registry.StatTypeCustom, Value: int32(registry.CustomStatJump)}
	mined := plugin.Statistic{Type: registry.StatTypeMined, Value: int32(registry.BlockStone)}
	env.Sync(func() {
		p.Handle().SetStatistic(jump, 5)
		p.Handle().SetStatistic(mined, 3)
	})
	b.Deposit(id, 12.5)
	tp(env, p, "other")
	env.Sync(func() {
		if j, m := p.Handle().Statistic(jump), p.Handle().Statistic(mined); j != 0 || m != 0 {
			t.Errorf("other world statistics: jump %d, mined stone %d", j, m)
		}
		p.Handle().SetStatistic(jump, 1)
	})
	if got := b.Balance(id); got != 0 {
		t.Errorf("balance in the other world %v, want 0", got)
	}
	b.Deposit(id, 2)
	tp(env, p, "world")
	env.Sync(func() {
		if j, m := p.Handle().Statistic(jump), p.Handle().Statistic(mined); j != 5 || m != 3 {
			t.Errorf("statistics back in world: jump %d, mined stone %d", j, m)
		}
	})
	if got := b.Balance(id); got != 12.5 {
		t.Errorf("balance back in world %v, want 12.5", got)
	}
	f := readFile(t, env, "worlds/world/Stat.json")
	if !strings.Contains(f, `"game_statistics":{"JUMP":5,"MINE_BLOCK":{"minecraft:stone":3}`) ||
		!strings.Contains(f, `"balance":"12.5"`) {
		t.Errorf("worlds/world/Stat.json = %s", f)
	}
	if f := readFile(t, env, "worlds/other/Stat.json"); !strings.Contains(f, `"balance":"2.0"`) {
		t.Errorf("worlds/other/Stat.json = %s", f)
	}
}

// ll:<world> teleports to where the player left a world, and to its spawn
// when they have no last location there.
func TestLastLocationDestination(t *testing.T) {
	env, _ := start(t, map[string]string{inv + "config.yml": `share-handling:
  active-optional-shares:
  - last_location
sharables:
  apply-last-location-for-all-teleports: false
first-run: false
`})
	createWorlds(env, "far")
	p := env.Join("Walker", true)
	tp(env, p, "far")
	env.Sync(func() {
		loc := p.Handle().Location()
		loc.Pos = plugin.Vec3{X: 7.5, Y: 70, Z: -3.5}
		p.Handle().Teleport(loc, plugin.TeleportCommand)
	})
	tp(env, p, "world")
	if pos := p.Handle().Location().Pos; pos.X == 7.5 {
		t.Fatalf("the last location applied to a plain teleport: %v", pos)
	}
	env.Expect(mvtest.Run(p, "mv tp ll:far"), "Teleported you to ll:far.")
	env.Srv.RunTicks(2)
	if loc := p.Handle().Location(); loc.Pos != (plugin.Vec3{X: 7.5, Y: 70, Z: -3.5}) {
		t.Errorf("ll:far took Walker to %v", loc.Pos)
	}
	env.Expect(mvtest.Run(p, "mv tp ll:nowhere"), "World 'nowhere' does not exist or is not loaded!")
}

// say chats as p and answers what they are sent.
func say(p *plugintest.Player, msg string) []string {
	p.Run(msg)
	return p.Messages(2)
}

// /mvinv group walks through creating a group by chat; "##" leaves it.
func TestGroupConversation(t *testing.T) {
	env, _ := start(t, map[string]string{inv + "config.yml": "first-run: false\n"})
	createWorlds(env, "c1")
	p := env.Join("Talker", true)
	env.Expect(mvtest.Run(p, "mvinv group"), "What would you like to do? Create, Edit or Delete.")
	env.Expect(say(p, "bogus"), "That is not a valid option!")
	env.Expect(say(p, "create"), "Please name your new group:")
	env.Expect(say(p, "bad-name"), "That name is not valid!")
	env.Expect(say(p, "talk"), "Enter the name of a world to add to group talk")
	env.Expect(say(p, "@"), "You may not have a group with no worlds")
	say(p, "c1")
	env.Expect(say(p, "@"), "or a specific share to add to group talk")
	env.Expect(say(p, "nonsense"), "You did not specify any valid shares!")
	say(p, "inventory")
	out := say(p, "@")
	env.Expect(out, "You created a new group 'talk'!")
	env.Expect(out, "Config Shares: inventory_contents, armor_contents, ender_chest, off_hand")
	if g := readFile(t, env, "groups.yml"); !strings.Contains(g, "  talk:\n    worlds:\n      - c1\n") {
		t.Errorf("groups.yml:\n%s", g)
	}
	mvtest.Run(p, "mvinv group")
	env.Expect(say(p, "##"), "##")
	if out := say(p, "hello"); mvtest.Has(out, "not a valid option") {
		t.Errorf("chat after ## still went to the conversation: %q", out)
	}
	env.Expect(env.Console("mvinv group"), "You are not allowed to access conversations")
}

// /mvinv view shows a stored inventory read-only; /mvinv modify saves the
// changes made in its menu.
func TestViewAndModifyMenus(t *testing.T) {
	env, _ := start(t, map[string]string{inv + "config.yml": "first-run: false\n"})
	createWorlds(env, "w2")
	tess := env.Join("Tess", false)
	tp(env, tess, "w2")
	env.Sync(func() { tess.Handle().Inventory().SetItem(0, plugin.NewStack(registry.ItemDiamond, 4)) })
	tp(env, tess, "world")
	admin := env.Join("Admin", true)

	top := func() plugin.Inventory {
		var inv plugin.Inventory
		env.Sync(func() {
			if v, ok := admin.Handle().OpenView(); ok {
				inv = v.Top()
			}
		})
		if inv == nil {
			t.Fatal("no menu is open")
		}
		return inv
	}
	out := mvtest.Run(admin, "mvinv view Tess w2")
	env.Expect(out, "Loading inventory data for Tess...")
	env.Expect(out, "Displaying STORED inventory for Tess in world w2. Changes will save on close.")
	inv := top()
	env.Sync(func() {
		if s := inv.Item(0); s.Item != registry.ItemDiamond || s.Count != 4 {
			t.Errorf("menu slot 0 = %+v", s)
		}
		if inv.Item(36).Item != registry.ItemGrayStainedGlassPane || inv.Item(41).Item != registry.ItemRedDye {
			t.Errorf("menu slots 36 and 41 = %v, %v", inv.Item(36).Item, inv.Item(41).Item)
		}
	})
	admin.Click(0, 0, 0)
	admin.Messages(2)
	env.Sync(func() {
		if !admin.Handle().Cursor().Empty() || inv.Item(0).Empty() {
			t.Error("the read-only menu let an item be taken")
		}
	})
	admin.CloseWindow()

	env.Expect(mvtest.Run(admin, "mvinv modify Tess w2"), "Displaying STORED inventory for Tess in world w2.")
	inv = top()
	admin.Click(0, 0, 0)
	admin.Click(5, 0, 0)
	admin.Click(41, 0, 0)
	admin.Messages(2)
	env.Sync(func() {
		if s := inv.Item(5); s.Item != registry.ItemDiamond || inv.Item(41).Item != registry.ItemRedDye {
			t.Errorf("modify menu slots 5 and 41 = %+v, %v", s, inv.Item(41).Item)
		}
	})
	admin.CloseWindow()
	admin.Messages(2)
	if f := readFile(t, env, "worlds/w2/Tess.json"); !strings.Contains(f, `"5":{"==":"org.bukkit.inventory.ItemStack","v":5023,"type":"DIAMOND","amount":4}`) ||
		strings.Contains(f, `"0":`) {
		t.Errorf("worlds/w2/Tess.json = %s", f)
	}
	tp(env, tess, "w2")
	env.Sync(func() {
		if s := tess.Handle().Inventory().Item(5); s.Item != registry.ItemDiamond {
			t.Errorf("Tess's slot 5 in w2 = %+v", s)
		}
	})
	env.Expect(mvtest.Run(admin, "mvinv modify Admin world"), "You cannot modify your own live inventory")
	env.Expect(env.Console("mvinv view Tess w2"), "This command can only be used by players")
}
