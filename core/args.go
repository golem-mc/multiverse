package core

import "github.com/golem-mc/golem/plugin"

// WorldArg resolves an issuer-aware world argument: Arg(i) when it names a
// world (by name or, with resolve-alias-name, alias), else the player's own
// world with the argument left for what follows. consumed reports whether
// Arg(i) was the world. loaded asks for a loaded world. MV:
// MVCommandContexts' MultiverseWorld and LoadedMultiverseWorld resolvers
func (ctx *Ctx) WorldArg(i int, loaded bool) (w *World, consumed bool, err error) {
	if arg := ctx.Arg(i); arg != "" {
		if w, ok := ctx.Core.Worlds.GetByNameOrAlias(arg); ok && (!loaded || w.Loaded()) {
			return w, true, nil
		}
	}
	if p, ok := ctx.Player(); ok {
		if w, ok := ctx.Core.Worlds.Of(p.World()); ok {
			return w, false, nil
		}
		if ctx.Arg(i) == "" {
			return nil, false, ctx.worldError(loaded, "issuer", "")
		}
	}
	if ctx.Arg(i) == "" {
		return nil, false, ctx.worldError(loaded, "playersonly", "")
	}
	if ctx.IsPlayer() {
		return nil, false, ctx.worldError(loaded, "input", ctx.Arg(i))
	}
	return nil, false, ctx.worldError(loaded, "inputconsole", ctx.Arg(i))
}

// RequireWorld resolves a required world argument at i, with no fallback to
// the player's world.
func (ctx *Ctx) RequireWorld(i int, loaded bool) (*World, error) {
	w, ok := ctx.Core.Worlds.GetByNameOrAlias(ctx.Arg(i))
	if !ok || loaded && !w.Loaded() {
		if ctx.IsPlayer() {
			return nil, ctx.worldError(loaded, "input", ctx.Arg(i))
		}
		return nil, ctx.worldError(loaded, "inputconsole", ctx.Arg(i))
	}
	return w, nil
}

func (ctx *Ctx) worldError(loaded bool, kind, input string) error {
	key := "mv-core.commands.error.multiverseworld."
	if loaded {
		key = "mv-core.commands.error.loadedmultiverseworld."
	}
	return InvalidArg(M(key+kind, "{world}", input))
}

// WorldNames completes world names: scope "loaded", "unloaded" or "both".
// MV: MVCommandCompletions @mvworlds
func (c *Core) WorldNames(scope string) []string {
	var out []string
	for _, w := range c.Worlds.All() {
		if scope == "loaded" && !w.Loaded() || scope == "unloaded" && w.Loaded() {
			continue
		}
		out = append(out, w.TabCompleteName())
	}
	return out
}

// PlayerNames completes online player names.
func (c *Core) PlayerNames(sender plugin.CommandSender) []string {
	var out []string
	for _, p := range c.Server.Players() {
		if viewer, ok := sender.(plugin.Player); ok && !viewer.CanSee(p) {
			continue
		}
		out = append(out, p.Name())
	}
	return out
}
