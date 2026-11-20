package commands

import (
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, gameruleCommands) }

// gameruleCommands are /mv gamerule set, reset and list. golem's game rules
// are shared by every dimension (golem-mc/golem#65), so setting one in a
// world sets it in all. MV: GameruleCommand
func gameruleCommands(c *core.Core) []*core.Cmd {
	worldsAndAll := func(ctx *core.Ctx) []string { return append(ctx.Core.WorldNames("loaded"), "*") }
	return []*core.Cmd{{
		Name: "gamerule set", Aliases: []string{"rule set"}, Legacy: []string{"mvrule", "mvgamerule"},
		Permission: "multiverse.core.gamerule.set", Syntax: "<gamerule> <value> [worlds|*]",
		Description: "mv-core.gamerule.set.description", MinArgs: 2, MaxArgs: 3,
		Complete: func(ctx *core.Ctx, pos int) []string {
			switch pos {
			case 0:
				return gameruleNames(ctx.Core)
			case 1:
				if r, ok := findGamerule(ctx.Core, ctx.Arg(0)); ok {
					if r.Boolean {
						return []string{"true", "false"}
					}
					return []string{strconv.Itoa(r.Default)}
				}
				return nil
			}
			return worldsAndAll(ctx)
		},
		Run: func(ctx *core.Ctx) error {
			r, err := gameruleArg(ctx, 0)
			if err != nil {
				return err
			}
			value, ok := parseGameruleValue(r, ctx.Arg(1))
			if !ok {
				return core.InvalidArg(core.Lit("The game rule value " + ctx.Arg(1) + " is not valid for game rule " + r.Key.Path + "."))
			}
			worlds, err := worldsArg(ctx, 2)
			if err != nil {
				return err
			}
			success := true
			for _, w := range worlds {
				h, ok := w.Handle()
				if !ok || value < r.Min || value > r.Max {
					ctx.Error(core.M("mv-core.gamerule.set.failed", "{gamerule}", r.Key.Path, "{value}", ctx.Arg(1),
						"{world}", w.Name(), "{type}", gameruleType(r)))
					success = false
					continue
				}
				h.SetGameRule(r.ID, value)
			}
			if success {
				shown := formatGameruleValue(r, value)
				if len(worlds) == 1 {
					ctx.Info(core.M("mv-core.gamerule.set.success.single", "{gamerule}", r.Key.Path, "{value}", shown, "{world}", worlds[0].Name()))
				} else if len(worlds) > 1 {
					ctx.Info(core.M("mv-core.gamerule.set.success.multiple", "{gamerule}", r.Key.Path, "{value}", shown, "{count}", len(worlds)))
				}
			}
			return nil
		},
	}, {
		Name: "gamerule reset", Aliases: []string{"rule reset"},
		Permission: "multiverse.core.gamerule.set", Syntax: "<gamerule> [worlds|*]",
		Description: "mv-core.gamerule.reset.description", MinArgs: 1, MaxArgs: 2,
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return gameruleNames(ctx.Core)
			}
			return worldsAndAll(ctx)
		},
		Run: func(ctx *core.Ctx) error {
			r, err := gameruleArg(ctx, 0)
			if err != nil {
				return err
			}
			worlds, err := worldsArg(ctx, 1)
			if err != nil {
				return err
			}
			success := true
			for _, w := range worlds {
				h, ok := w.Handle()
				if !ok {
					ctx.Error(core.M("mv-core.gamerule.reset.failed", "{gamerule}", r.Key.Path, "{world}", w.Name()))
					success = false
					continue
				}
				h.SetGameRule(r.ID, r.Default)
			}
			if success {
				if len(worlds) == 1 {
					ctx.Info(core.M("mv-core.gamerule.reset.success.single", "{gamerule}", r.Key.Path, "{world}", worlds[0].Name()))
				} else if len(worlds) > 1 {
					ctx.Info(core.M("mv-core.gamerule.reset.success.multiple", "{gamerule}", r.Key.Path, "{count}", len(worlds)))
				}
			}
			return nil
		},
	}, {
		Name: "gamerule list", Aliases: []string{"rule list"}, Legacy: []string{"mvrules", "mvgamerules"},
		Permission: "multiverse.core.gamerule.list", Syntax: "[world] [--page <page>] [--filter <filter>]",
		Description: "mv-core.gamerule.list.description", MaxArgs: 1, Flags: core.PageFilterFlags(),
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("loaded") },
		Run: func(ctx *core.Ctx) error {
			w, err := optionalWorld(ctx, 0, true)
			if err != nil {
				return err
			}
			d, err := ctx.PagedDisplay()
			if err != nil {
				return err
			}
			h := core.M("mv-core.gamerule.list.title", "{world}", w.Name())
			d.Header = &h
			var keys []string
			vals := map[string]*string{}
			if handle, ok := w.Handle(); ok {
				for _, r := range ctx.Core.Server.Registries().GameRules() {
					v := formatGameruleValue(r, handle.GameRule(r.ID))
					keys = append(keys, r.Key.Path)
					vals[r.Key.Path] = &v
				}
			}
			slices.Sort(keys)
			d.Send(ctx.Issuer, core.MapLines(ctx.Issuer, keys, vals, "§b", "§f"))
			return nil
		},
	}}
}

// gameruleNames are the game rules' names: their key paths.
func gameruleNames(c *core.Core) []string {
	var out []string
	for _, r := range c.Server.Registries().GameRules() {
		out = append(out, r.Key.Path)
	}
	return out
}

// findGamerule finds a game rule by key ("minecraft:keep_inventory"), path
// ("keep_inventory") or, for an admin used to Bukkit's names, with case and
// underscores ignored ("keepInventory").
func findGamerule(c *core.Core, name string) (plugin.GameRuleInfo, bool) {
	simple := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "")) }
	for _, r := range c.Server.Registries().GameRules() {
		if name == r.Key.String() || name == r.Key.Path || simple(name) == simple(r.Key.Path) {
			return r, true
		}
	}
	return plugin.GameRuleInfo{}, false
}

// gameruleArg resolves the game rule at i. MV: MVCommandContexts.parseGameRule
func gameruleArg(ctx *core.Ctx, i int) (plugin.GameRuleInfo, error) {
	if ctx.Arg(i) == "" {
		return plugin.GameRuleInfo{}, core.InvalidArg(core.Lit("No game rule specified."))
	}
	r, ok := findGamerule(ctx.Core, ctx.Arg(i))
	if !ok {
		return r, core.InvalidArg(core.Lit("The game rule " + ctx.Arg(i) + " is not valid."))
	}
	return r, nil
}

// parseGameruleValue reads a value for r: true or false for a boolean rule
// (golem keeps it as 1 or 0), an integer otherwise.
func parseGameruleValue(r plugin.GameRuleInfo, s string) (int, bool) {
	if r.Boolean {
		switch strings.ToLower(s) {
		case "true":
			return 1, true
		case "false":
			return 0, true
		}
		return 0, false
	}
	v, err := strconv.Atoi(s)
	return v, err == nil
}

func formatGameruleValue(r plugin.GameRuleInfo, v int) string {
	if r.Boolean {
		return strconv.FormatBool(v != 0)
	}
	return strconv.Itoa(v)
}

// gameruleType names a rule's type as Bukkit's GameRule.getType does.
func gameruleType(r plugin.GameRuleInfo) string {
	if r.Boolean {
		return "java.lang.Boolean"
	}
	return "java.lang.Integer"
}
