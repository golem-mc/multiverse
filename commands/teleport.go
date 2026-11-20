package commands

import (
	"fmt"
	"strings"

	"github.com/golem-mc/golem/plugin"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, teleportCommands) }

var (
	unsafeFlag = &core.Flag{Name: "--unsafe", Aliases: []string{"-u"}}
	silentFlag = &core.Flag{Name: "--silent", Aliases: []string{"-s"}}
)

// teleportCommands move players and set where they go. MV: TeleportCommand,
// SpawnCommand, SetSpawnCommand, CheckCommand, AnchorSetCommand,
// AnchorDeleteCommand, AnchorListCommand
func teleportCommands(c *core.Core) []*core.Cmd {
	c.Commands.SetDynamicPermission("@mvteleport", func(s plugin.CommandSender) bool { return c.HasAnyTeleport(s) })
	c.Commands.SetDynamicPermission("@mvspawn", func(s plugin.CommandSender) bool { return c.HasAnySpawn(s) })
	return []*core.Cmd{{
		Name: "teleport", Aliases: []string{"tp"}, Roots: []string{"mvtp"}, Permission: "@mvteleport",
		Syntax: "[player] <destination> [--unsafe --silent]", Description: "mv-core.teleport.description",
		MinArgs: 1, MaxArgs: 2, Flags: []*core.Flag{unsafeFlag, silentFlag},
		Complete: completeTeleport, Run: runTeleport,
	}, {
		Name: "spawn", Roots: []string{"mvspawn"}, Permission: "@mvspawn",
		Syntax: "[player] [--unsafe]", Description: "mv-core.spawn.description", MaxArgs: 1,
		Flags: []*core.Flag{unsafeFlag},
		Complete: func(ctx *core.Ctx, _ int) []string {
			if ctx.Core.HasAnySpawn(ctx.Sender, core.ScopeOther) {
				return ctx.Core.PlayerNames(ctx.Sender)
			}
			return nil
		},
		Run: runSpawn,
	}, {
		Name: "setspawn", Roots: []string{"mvsetspawn"}, Legacy: []string{"mvss"},
		Permission: "multiverse.core.spawn.set", Syntax: "[worldname:x,y,z[,yaw,pitch]] [--unsafe]",
		Description: "mv-core.setspawn.description", MaxArgs: 1, Flags: []*core.Flag{unsafeFlag},
		Run: runSetSpawn,
	}, {
		Name: "check", Legacy: []string{"mvcheck"}, Permission: "multiverse.core.check",
		Syntax: "<player> <destination>", Description: "mv-core.check.description", MinArgs: 2, MaxArgs: 2,
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return ctx.Core.PlayerNames(ctx.Sender)
			}
			return ctx.Core.Destinations.Suggest(ctx.Sender, ctx.Arg(1))
		},
		Run: func(ctx *core.Ctx) error {
			p, ok := ctx.Core.FindPlayer(ctx.Sender, ctx.Arg(0))
			if !ok {
				return core.InvalidArg(core.M("mv-core.commands.error.player.input", "{player}", ctx.Arg(0)))
			}
			d, err := ctx.Core.Destinations.Parse(ctx.Sender, ctx.Arg(1))
			if err != nil {
				return core.InvalidArg(core.ErrorMessage(err))
			}
			key := "mv-core.check.nopermission"
			if ctx.Core.CanTeleport(p, []plugin.Entity{p}, d) {
				key = "mv-core.check.haspermission"
			}
			ctx.Info(core.M(key, "{player}", p.Name(), "{destination}", core.DestinationString(d)))
			loc := core.M("mv-core.generic.null")
			if l, ok := d.Location(p); ok && l.World != nil {
				loc = core.Lit(core.LocationString(worldName(ctx.Core, l.World), l))
			}
			ctx.Info(core.M("mv-core.check.location", "{location}", loc))
			return nil
		},
	}, {
		Name: "anchor set", Permission: "multiverse.core.anchor.create", Syntax: "<name> [location]",
		MinArgs: 1, MaxArgs: 2,
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return anchorNames(ctx)
			}
			return nil
		},
		Run: func(ctx *core.Ctx) error {
			name := ctx.Arg(0)
			var loc core.WorldLocation
			if len(ctx.Args) == 2 {
				l, ok := core.ParseLocationString(ctx.Arg(1))
				if !ok {
					ctx.Send(core.M("mv-core.anchor.set.failure", "{anchor}", name))
					return nil
				}
				loc = l
			} else {
				p, ok := ctx.Player()
				if !ok {
					return core.InvalidArg(core.M("mv-core.commands.error.playersonly"))
				}
				l := p.Location()
				loc, _ = core.ParseLocationString(core.LocationString(worldName(ctx.Core, l.World), l))
			}
			if err := ctx.Core.Anchors.Set(name, loc); err != nil {
				ctx.Send(core.M("mv-core.anchor.set.failure", "{anchor}", name))
				return nil
			}
			ctx.Send(core.M("mv-core.anchor.set.success", "{anchor}", name, "{location}", loc.String()))
			return nil
		},
	}, {
		Name: "anchor delete", Permission: "multiverse.core.anchor.delete", Syntax: "<name>", MinArgs: 1, MaxArgs: 1,
		Complete: func(ctx *core.Ctx, _ int) []string { return anchorNames(ctx) },
		Run: func(ctx *core.Ctx) error {
			a, ok := ctx.Core.Anchors.Get(ctx.Arg(0))
			if !ok {
				return core.InvalidArg(core.Lit("The anchor '" + ctx.Arg(0) + "' does not exist."))
			}
			if err := ctx.Core.Anchors.Delete(a.Name); err != nil {
				ctx.Error(core.M("mv-core.anchor.delete.failure"))
				return nil
			}
			ctx.Send(core.M("mv-core.anchor.delete.success", "{anchor}", a.Name))
			return nil
		},
	}, {
		Name: "anchor list", Permission: "multiverse.core.anchor.list", Syntax: "[--page <page>] [--filter <filter>]",
		Flags: core.PageFilterFlags(),
		Run: func(ctx *core.Ctx) error {
			d, err := ctx.PagedDisplay()
			if err != nil {
				return err
			}
			h := core.M("mv-core.anchor.list.header")
			d.Header = &h
			var lines []string
			for _, a := range ctx.Core.Anchors.For(ctx.Sender) {
				lines = append(lines, ctx.Format(core.M("mv-core.anchor.list.entry", "{anchor}", a.Name,
					"{location}", a.Location.String())))
			}
			d.Send(ctx.Issuer, lines)
			return nil
		},
	}}
}

func anchorNames(ctx *core.Ctx) []string {
	var out []string
	for _, a := range ctx.Core.Anchors.For(ctx.Sender) {
		out = append(out, a.Name)
	}
	return out
}

// worldName answers the name a world goes by in Multiverse's messages.
func worldName(c *core.Core, h plugin.World) string {
	if w, ok := c.Worlds.Of(h); ok {
		return w.Name()
	}
	return h.Key().String()
}

// teleportees resolves /mv tp's first argument: the players it names, or
// with one argument the issuer. ok is false when the argument was the
// players, so the destination is the second. MV: issuer-aware
// PlayerArrayValue with maxArgForAware=1
func teleportees(ctx *core.Ctx, maxForSelf int) (ps []plugin.Player, consumed bool, err error) {
	in := ctx.Arg(0)
	if in != "" {
		found, err := ctx.Core.FindPlayers(ctx.Sender, in)
		if err != nil {
			return nil, false, core.InvalidArg(core.ErrorMessage(err))
		}
		if len(found) > 0 {
			return found, true, nil
		}
	}
	if len(ctx.Args) <= maxForSelf {
		if p, ok := ctx.Player(); ok {
			return []plugin.Player{p}, false, nil
		}
		if in == "" {
			return nil, false, core.InvalidArg(core.M("mv-core.commands.error.playersonly"))
		}
		return nil, false, core.InvalidArg(core.M("mv-core.commands.error.player.issuerinput", "{player}", in))
	}
	if strings.HasPrefix(in, "@") {
		return nil, false, core.InvalidArg(core.M("mv-core.commands.error.player.selector", "{player}", in))
	}
	return nil, false, core.InvalidArg(core.M("mv-core.commands.error.players.input", "{player}", in))
}

func entities(ps []plugin.Player) []plugin.Entity {
	out := make([]plugin.Entity, len(ps))
	for i, p := range ps {
		out[i] = p
	}
	return out
}

func isSelf(ctx *core.Ctx, p plugin.Player) bool {
	self, ok := ctx.Player()
	return ok && self.UUID() == p.UUID()
}

// youOr answers "you" when p is the issuer, else p's name.
func youOr(ctx *core.Ctx, p plugin.Player) core.Message {
	if isSelf(ctx, p) {
		return core.M("mv-core.generic.you")
	}
	return core.Lit(p.Name())
}

// runTeleport is /mv tp. MV: TeleportCommand.onTeleportCommand
func runTeleport(ctx *core.Ctx) error {
	ps, consumed, err := teleportees(ctx, 1)
	if err != nil {
		return err
	}
	destArg := ctx.Arg(0)
	if consumed {
		destArg = ctx.Arg(1)
	}
	if destArg == "" {
		return core.ErrUsage
	}
	d, err := ctx.Core.Destinations.Parse(ctx.Sender, destArg)
	if err != nil {
		return core.InvalidArg(core.ErrorMessage(err))
	}
	limit := ctx.Core.Config.Int("teleport.concurrent-teleport-limit")
	if len(ps) > 1 && len(ps) > limit {
		ctx.Error(core.M("mv-core.teleport.toomanyplayers", "{count}", limit))
		return nil
	}
	es := entities(ps)
	if !ctx.Core.CanTeleport(ctx.Sender, es, d) {
		switch {
		case len(ps) > 1:
			ctx.Error(core.M("mv-core.teleport.nopermission.all"))
		case isSelf(ctx, ps[0]):
			ctx.Error(core.M("mv-core.teleport.nopermission.self"))
		default:
			ctx.Error(core.M("mv-core.teleport.nopermission.other"))
		}
		return nil
	}
	t := ctx.Core.TeleportTo(d)
	t.Safety = !ctx.Has("--unsafe") && d.CheckSafety()
	t.By = ctx.Sender
	silent := ctx.Has("--silent")
	if len(ps) == 1 {
		p := ps[0]
		t.Run(p, func(f core.TeleportFailure) {
			if f != "" {
				ctx.Error(core.M("mv-core.teleport.failed", "{player}", p.Name(), "{destination}", d.Display(),
					"{reason}", f.Message()))
				return
			}
			if !silent {
				ctx.Info(core.M("mv-core.teleport.success", "{player}", youOr(ctx, p), "{destination}", d.Display()))
			}
		})
		return nil
	}
	t.RunAll(es, func(ok int, failed map[core.TeleportFailure]int) {
		if !silent {
			ctx.Info(core.M("mv-core.teleport.success", "{player}", core.M("mv-core.generic.playercount", "{count}", ok),
				"{destination}", d.Display()))
		}
		for f, n := range failed {
			ctx.Error(core.M("mv-core.teleport.failed", "{player}", core.M("mv-core.generic.playercount", "{count}", n),
				"{destination}", d.Display(), "{reason}", f.Message()))
		}
	})
	return nil
}

// completeTeleport completes /mv tp: players (with permission to move
// others) or the issuer's destinations first, then the named players'
// destinations.
func completeTeleport(ctx *core.Ctx, pos int) []string {
	c := ctx.Core
	suggest := func(ps []plugin.Player, input string) []string {
		var out []string
		simplified := c.Config.Bool("command.simplified-destination-tab-completion")
		for _, s := range c.Destinations.SuggestAll(ctx.Sender, input) {
			if simplified && s.Type.ID() != "w" && !strings.HasPrefix(input, s.Type.ID()+":") {
				continue
			}
			if c.CanSuggest(ctx.Sender, entities(ps), s) {
				out = append(out, s.String())
			}
		}
		return out
	}
	if pos == 0 {
		var out []string
		if c.HasAnyTeleport(ctx.Sender, core.ScopeOther) {
			out = c.PlayerNames(ctx.Sender)
		}
		if p, ok := ctx.Player(); ok {
			out = append(out, suggest([]plugin.Player{p}, ctx.Arg(0))...)
		}
		return out
	}
	ps, err := c.FindPlayers(ctx.Sender, ctx.Arg(0))
	if err != nil || len(ps) == 0 {
		return nil
	}
	return suggest(ps, ctx.Arg(1))
}

// runSpawn is /mv spawn: each player to their own world's spawn.
// MV: SpawnCommand.onSpawnTpCommand
func runSpawn(ctx *core.Ctx) error {
	ps, _, err := teleportees(ctx, 1)
	if err != nil {
		return err
	}
	byWorld := map[plugin.Key][]plugin.Player{}
	var order []plugin.Key
	for _, p := range ps {
		k := p.World().Key()
		if byWorld[k] == nil {
			order = append(order, k)
		}
		byWorld[k] = append(byWorld[k], p)
	}
	for _, k := range order {
		group := byWorld[k]
		h := group[0].World()
		w, ok := ctx.Core.Worlds.Of(h)
		if !ok || !w.Loaded() {
			ctx.Send(core.M("mv-core.spawn.notmvworld", "{world}", worldName(ctx.Core, h)))
			continue
		}
		es := entities(group)
		if !ctx.Core.CanSpawn(ctx.Sender, es, w) {
			ctx.Send(core.M("mv-core.spawn.nopermission"))
			continue
		}
		spawn, _ := w.Spawn()
		t := ctx.Core.TeleportToLocation(spawn)
		t.Safety = !ctx.Has("--unsafe")
		t.By = ctx.Sender
		if len(group) == 1 {
			p := group[0]
			t.Run(p, func(f core.TeleportFailure) {
				if f != "" {
					ctx.Error(core.M("mv-core.spawn.failed", "{player}", youOr(ctx, p), "{world}", w.Name(),
						"{reason}", f.Message()))
					return
				}
				ctx.Info(core.M("mv-core.spawn.success", "{player}", youOr(ctx, p), "{world}", w.Name()))
			})
			continue
		}
		t.RunAll(es, func(ok int, failed map[core.TeleportFailure]int) {
			ctx.Send(core.M("mv-core.spawn.success", "{player}", core.M("mv-core.generic.playercount", "{count}", ok),
				"{world}", w.Name()))
			for f, n := range failed {
				ctx.Error(core.M("mv-core.spawn.failed", "{player}", core.M("mv-core.generic.playercount", "{count}", n),
					"{world}", w.Name(), "{reason}", f.Message()))
			}
		})
	}
	return nil
}

// runSetSpawn is /mv setspawn. MV: SetSpawnCommand.onSetSpawnCommand
func runSetSpawn(ctx *core.Ctx) error {
	loc, err := playerLocation(ctx)
	if err != nil {
		return err
	}
	if !ctx.Has("--unsafe") && !core.SafeAt(loc.World, loc.Pos.Block()) {
		ctx.Send(core.M("mv-core.setspawn.unsafe"))
		return nil
	}
	w, ok := ctx.Core.Worlds.Of(loc.World)
	if !ok || !w.Loaded() {
		ctx.Send(core.M("mv-core.setspawn.notmvworld", "{world}", worldName(ctx.Core, loc.World)))
		return nil
	}
	ctx.Core.Worlds.SetSpawn(w, loc)
	pretty := fmt.Sprintf("%.2f, %.2f, %.2f, pitch:%.2f, yaw:%.2f", loc.Pos.X, loc.Pos.Y, loc.Pos.Z, loc.Pitch, loc.Yaw)
	if err := ctx.Core.Worlds.Save(); err != nil {
		ctx.Send(core.M("mv-core.setspawn.failed", "{world}", w.Name(), "{location}", pretty, "{error}", err.Error()))
		return nil
	}
	ctx.Send(core.M("mv-core.setspawn.success", "{world}", w.Name(), "{location}", pretty))
	return nil
}

// playerLocation reads setspawn's location: "[world:]x,y,z[,yaw,pitch]"
// ("~" makes it relative to the player), else where the player stands.
// MV: MVCommandContexts.parsePlayerLocation
func playerLocation(ctx *core.Ctx) (plugin.Location, error) {
	p, isPlayer := ctx.Player()
	in := ctx.Arg(0)
	if in == "" {
		if isPlayer {
			return p.Location(), nil
		}
		return plugin.Location{}, core.InvalidArg(core.Lit("You must specify a world location when using this command in console."))
	}
	worldPart, rest, hasWorld := strings.Cut(in, ":")
	var base plugin.Location
	var h plugin.World
	switch {
	case hasWorld:
		w, ok := ctx.Core.Worlds.GetLoaded(worldPart)
		if !ok {
			return plugin.Location{}, core.InvalidArg(core.Lit("That world does not exist."))
		}
		h, _ = w.Handle()
	case isPlayer:
		base, rest, h = p.Location(), in, p.World()
	default:
		return plugin.Location{}, core.InvalidArg(core.Lit("You must specify a world location when using this command in console."))
	}
	rel := strings.HasPrefix(rest, "~")
	if rel && !isPlayer {
		return plugin.Location{}, core.InvalidArg(core.Lit("You may not use relative coordinates from console."))
	}
	if rel {
		base = p.Location()
	}
	parts := strings.Split(strings.TrimPrefix(rest, "~"), ",")
	bad := core.InvalidArg(core.Lit("Please specify the coordinates x, y and z."))
	if len(parts) < 3 {
		return plugin.Location{}, bad
	}
	var v [5]float64
	for i := range min(len(parts), 5) {
		if parts[i] == "" && rel && i < 3 {
			continue
		}
		var f float64
		if _, err := fmt.Sscan(parts[i], &f); err != nil {
			return plugin.Location{}, bad
		}
		v[i] = f
	}
	loc := plugin.Location{World: h, Pos: plugin.Vec3{X: v[0], Y: v[1], Z: v[2]}}
	if rel {
		loc.Pos = loc.Pos.Add(base.Pos)
	}
	if len(parts) >= 5 {
		loc.Yaw, loc.Pitch = float32(v[3]), float32(v[4])
	}
	return loc, nil
}
