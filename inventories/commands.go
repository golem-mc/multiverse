package inventories

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
)

const (
	importerURL     = "https://modrinth.com/project/multiverse-inventoriesimporter/"
	importURL       = "https://mvplugins.org/inventories/how-to/import-playerdata/"
	lastLocationURL = "https://mvplugins.org/inventories/how-to/configure-last-location/"
	// maxGive is the most items /mvinv give gives at once.
	maxGive = 6400
)

// commands are /mvinv's subcommands. MV: the commands package
func (m *Module) commands() []*core.Cmd {
	cmds := []*core.Cmd{{
		Name: "add-disabled-shares", Permission: "multiverse.inventories.adddisabledshares",
		Syntax: "<group> <share[,extra]>", Description: "Add one or more disabled shares to a group.",
		MinArgs: 2, MaxArgs: 2, Complete: m.completeGroupShares,
		Run: m.editShares(func(g *WorldGroup, sh *Shares) { g.disabled.AddAll(sh) }, true),
	}, {
		Name: "add-shares", Permission: "multiverse.inventories.addshares",
		Syntax: "<group> <share[,extra]>", Description: "Add one or more shares to a group.",
		MinArgs: 2, MaxArgs: 2, Complete: m.completeGroupShares,
		Run: m.editShares(func(g *WorldGroup, sh *Shares) { g.shares.AddAll(sh) }, false),
	}, {
		Name: "add-worlds", Permission: "multiverse.inventories.addworlds",
		Syntax: "<group> <world[,extra]> [--skip-exist-check]", Description: "Adds a World to a World Group.",
		MinArgs: 2, MaxArgs: 2, Flags: []*core.Flag{{Name: "--skip-exist-check", Aliases: []string{"-s"}}},
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return m.groupNames()
			}
			return addonToCommaSeparated(lastWord(ctx), m.Core.WorldNames("both"))
		},
		Run: m.addWorlds,
	}, {
		Name: "cache invalidate all", Permission: "multiverse.inventories.cache.invalidate",
		Run: func(*core.Ctx) error { m.store.clear(); return nil },
	}, {
		Name: "cache invalidate player", Permission: "multiverse.inventories.cache.invalidate",
		Syntax: "<player>", MaxArgs: 1,
		Complete: func(ctx *core.Ctx, _ int) []string { return m.Core.PlayerNames(ctx.Sender) },
		Run: func(ctx *core.Ctx) error {
			p, err := m.issuerAwarePlayer(ctx, 0)
			if err == nil {
				m.store.forgetPlayer(p.UUID())
			}
			return err
		},
	}, {
		Name: "cache stats", Permission: "multiverse.inventories.cache.stats",
		Run: m.cacheStats,
	}, {
		Name: "check-group-conflicts", Permission: "multiverse.inventories.checkgroupconflict",
		Description: "Check for conflicts in World Groups.",
		Run:         func(ctx *core.Ctx) error { m.sendConflicts(ctx.Issuer, true); return nil },
	}, {
		Name: "config", Permission: "multiverse.inventories.config",
		Syntax: "<name> [value]", Description: "Show or set a config value.", MinArgs: 1, MaxArgs: 2,
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return m.Config.Nodes().Names()
			}
			if n, ok := m.Config.Nodes().Find(ctx.Arg(0)); ok {
				return core.SuggestValues(n, lastWord(ctx))
			}
			return nil
		},
		Run: m.config,
	}, {
		Name: "create-group", Permission: "multiverse.inventories.creategroup",
		Syntax:      "<name> [share[,extra]] [world[,extra]]",
		Description: "Creates a new empty World Group with no worlds and no shares.", MinArgs: 1, MaxArgs: 3,
		Complete: func(ctx *core.Ctx, pos int) []string {
			switch pos {
			case 1:
				return addonToCommaSeparated(lastWord(ctx), m.Core.WorldNames("both"))
			case 2:
				return suggestShares(lastWord(ctx))
			}
			return nil
		},
		Run: m.createGroup,
	}, {
		Name: "delete-group", Permission: "multiverse.inventories.deletegroup",
		Syntax: "<group>", Description: "Deletes a World Group.", MinArgs: 1, MaxArgs: 1,
		Complete: func(*core.Ctx, int) []string { return m.groupNames() },
		Run: func(ctx *core.Ctx) error {
			g, err := m.groupArg(ctx, 0)
			if err != nil {
				return err
			}
			ctx.Core.Commands.Confirm(ctx.Issuer, core.M("mv-inventories.deletegroup.confirmprompt", "{group}", g.name), func() {
				m.Groups.Remove(g)
				ctx.Info(core.M("mv-inventories.deletegroup.success", "{group}", g.name))
			})
			m.sendConflicts(ctx.Issuer, false)
			return nil
		},
	}, {
		Name: "give", Permission: "multiverse.inventories.give",
		Syntax: "<player> <world> [gamemode] <item> [amount]", Description: "World and Group Information",
		MinArgs: 3, MaxArgs: -1, Complete: m.completeGive, Run: m.give,
	}, {
		Name: "list", Legacy: []string{"mvinvlist", "mvinvl"}, Permission: "multiverse.inventories.list",
		Description: "World and Group Information",
		Run: func(ctx *core.Ctx) error {
			names := "N/A"
			if gs := m.groupNames(); len(gs) > 0 {
				names = strings.Join(gs, ", ")
			}
			ctx.Info(core.M("mv-inventories.list.groups"))
			ctx.Info(core.M("mv-inventories.list.groups.info", "{groups}", names))
			return nil
		},
	}, {
		Name: "info", Legacy: []string{"mvinvinfo", "mvinvi"}, Permission: "multiverse.inventories.info",
		Syntax: "<world|group>", Description: "World and Group Information", MaxArgs: 1,
		Complete: func(*core.Ctx, int) []string { return append(m.Core.WorldNames("both"), m.groupNames()...) },
		Run:      m.info,
	}, {
		Name: "migrate", Permission: "multiverse.inventories.migrate",
		Syntax:      "<MultiInv|WorldInventories|PerWorldInventory>",
		Description: "Import inventories from MultiInv/WorldInventories/PerWorldInventory plugin.", MinArgs: 1, MaxArgs: 1,
		Run: func(ctx *core.Ctx) error {
			ctx.Error(core.M("mv-inventories.migrate.importernotinstalled"))
			ctx.Info(core.M("mv-inventories.migrate.downloadlink", "{url}", importerURL))
			ctx.Info(core.M("mv-inventories.migrate.learnmore", "{url}", importURL))
			return nil
		},
	}, {
		Name: "playerdata import", Permission: "multiverse.inventories.importplayerdata",
		Syntax: "<world>", Description: "Import player data from the world's playerdata folder.", MinArgs: 1, MaxArgs: 1,
		Complete: func(*core.Ctx, int) []string { return m.Core.WorldNames("loaded") },
		Run: func(ctx *core.Ctx) error {
			ctx.Error(core.M("mv-inventories.playerdataimport.importernotinstalled"))
			ctx.Info(core.M("mv-inventories.playerdataimport.downloadlink", "{url}", importerURL))
			ctx.Info(core.M("mv-inventories.playerdataimport.learnmore", "{url}", importURL))
			return nil
		},
	}, {
		Name: "reload", Legacy: []string{"mvinvreload"}, Permission: "multiverse.inventories.reload",
		Description: "Reloads config file",
		Run: func(ctx *core.Ctx) error {
			if err := m.reload(); err != nil {
				return err
			}
			ctx.Info(core.M("mv-inventories.reload.complete"))
			return nil
		},
	}, {
		Name: "remove-disabled-shares", Permission: "multiverse.inventories.removedisabledshares",
		Syntax: "<group> <share[,extra]>", Description: "Remove one or more disabled shares from a group.",
		MinArgs: 2, MaxArgs: 2, Complete: m.completeGroupShares,
		Run: m.editShares(func(g *WorldGroup, sh *Shares) { g.disabled.RemoveAll(sh) }, true),
	}, {
		Name: "remove-shares", Permission: "multiverse.inventories.removeshares",
		Syntax: "<group> <share[,extra]>", Description: "Remove one or more shares from a group.",
		MinArgs: 2, MaxArgs: 2, Complete: m.completeGroupShares,
		Run: m.editShares(func(g *WorldGroup, sh *Shares) { g.shares.RemoveAll(sh) }, false),
	}, {
		Name: "remove-worlds", Permission: "multiverse.inventories.removeworlds",
		Syntax: "<group>, <world[,extra]>", Description: "Adds a World to a World Group.", MinArgs: 2, MaxArgs: 2,
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return m.groupNames()
			}
			if g, ok := m.Groups.Get(ctx.Arg(0)); ok {
				return addonToCommaSeparated(lastWord(ctx), g.configWorlds)
			}
			return nil
		},
		Run: func(ctx *core.Ctx) error {
			g, err := m.groupArg(ctx, 0)
			if err != nil {
				return err
			}
			if !g.RemoveWorlds(strings.Split(ctx.Arg(1), ","), true) {
				ctx.Error(core.M("mv-inventories.removeworld.worldnotingroup", "{group}", g.name, "{world}", ctx.Arg(1)))
				return nil
			}
			ctx.Info(core.M("mv-inventories.removeworld.worldremoved", "{group}", g.name, "{world}", ctx.Arg(1)))
			m.sendConflicts(ctx.Issuer, false)
			return nil
		},
	}, {
		Name: "toggle", Legacy: []string{"mvinvtoggle"}, Permission: "multiverse.inventories.addshares",
		Syntax: "<share-name>", Description: "Toggles the usage of optional sharables", MinArgs: 1, MaxArgs: 1,
		Complete: func(*core.Ctx, int) []string { return optionalShares.names() },
		Run:      m.toggle,
	}}
	cmds = append(cmds, m.bulkEditCommands()...)
	cmds = append(cmds, m.viewCommands()...)
	help := &core.Cmd{
		Name: "help", Permission: "multiverse.inventories.help", Syntax: "[filter] [page]",
		Description: "Displays a list of available commands.", MaxArgs: 2,
	}
	help.Complete = func(_ *core.Ctx, pos int) []string {
		if pos > 0 {
			return nil
		}
		var out []string
		for _, cmd := range cmds {
			out = append(out, strings.Fields(cmd.Name)[0])
		}
		return out
	}
	help.Run = func(ctx *core.Ctx) error {
		filter, page := ctx.Arg(0), 1
		if n, err := strconv.Atoi(filter); err == nil {
			filter, page = "", n
		} else if n, err := strconv.Atoi(ctx.Arg(1)); err == nil {
			page = n
		}
		ctx.Core.Commands.Help(ctx.Issuer, "mvinv", append(cmds, help), filter, page)
		return nil
	}
	return append(cmds, help)
}

// registerCommands registers /mvinv and its legacy aliases.
func (m *Module) registerCommands() error {
	for _, node := range []string{"multiverse.inventories.import", "multiverse.inventories.spawn.self",
		"multiverse.inventories.spawn.other", "multiverse.inventories.debug"} {
		m.Core.Permissions.Register(node, core.DefaultOp)
	}
	return m.Core.Commands.RegisterRoot("mvinv", nil, m.commands())
}

// lastWord answers the word being completed.
func lastWord(ctx *core.Ctx) string {
	if len(ctx.Words) == 0 {
		return ""
	}
	return ctx.Words[len(ctx.Words)-1]
}

// addonToCommaSeparated completes the last entry of a comma-separated list.
// MV: StringFormatter.addonToCommaSeperated
func addonToCommaSeparated(input string, options []string) []string {
	i := strings.LastIndex(input, ",")
	if i < 0 {
		return options
	}
	out := make([]string, len(options))
	for j, o := range options {
		out[j] = input[:i+1] + o
	}
	return out
}

// javaList writes names as Java's List.toString does.
func javaList(names []string) string { return "[" + strings.Join(names, ", ") + "]" }

func (m *Module) groupNames() []string {
	var out []string
	for _, g := range m.Groups.All() {
		out = append(out, g.name)
	}
	return out
}

// groupArg answers the group argument i names. MV: MVInvCommandContexts.parseWorldGroup
func (m *Module) groupArg(ctx *core.Ctx, i int) (*WorldGroup, error) {
	g, ok := m.Groups.Get(ctx.Arg(i))
	if !ok {
		return nil, core.InvalidArg(core.M("mv-inventories.error.nogroup", "{group}", ctx.Arg(i)))
	}
	return g, nil
}

// sharesArg reads a comma-separated list of shares, a "-" taking one out.
// MV: MVInvCommandContexts.parseShares
func sharesArg(s string) (*Shares, error) {
	add, remove := &Shares{}, &Shares{}
	for name := range strings.SplitSeq(s, ",") {
		to := add
		if rest, ok := strings.CutPrefix(name, "-"); ok {
			name, to = rest, remove
		}
		if sh, ok := LookupShares(name); ok {
			to.AddAll(sh)
		}
	}
	add.RemoveAll(remove)
	if add.Empty() {
		return nil, core.InvalidArg(core.M("mv-inventories.error.nosharesspecified"))
	}
	return add, nil
}

// suggestShares completes a comma-separated list of shares.
// MV: MVInvCommandCompletion.suggestShares
func suggestShares(input string) []string {
	names := ShareNames()
	i := strings.LastIndex(input, ",")
	if i < 0 {
		if strings.HasPrefix(input, "-") {
			out := make([]string, len(names))
			for j, n := range names {
				out[j] = "-" + n
			}
			return out
		}
		return names
	}
	prefix := input[:i+1]
	if strings.HasPrefix(input[i+1:], "-") {
		prefix += "-"
	}
	current := map[string]bool{}
	for s := range strings.SplitSeq(input, ",") {
		current[strings.TrimPrefix(s, "-")] = true
	}
	var out []string
	for _, n := range names {
		if !current[n] {
			out = append(out, prefix+n)
		}
	}
	return out
}

func (sh *Shares) names() []string {
	out := make([]string, len(sh.list))
	for i, s := range sh.list {
		out[i] = s.Name()
	}
	return out
}

func (m *Module) completeGroupShares(ctx *core.Ctx, pos int) []string {
	if pos == 0 {
		return m.groupNames()
	}
	return suggestShares(lastWord(ctx))
}

// info shows a world's groups and a group's worlds and shares.
// MV: InfoCommand
func (m *Module) info(ctx *core.Ctx) error {
	name := ctx.Arg(0)
	if name == "" {
		p, ok := ctx.Player()
		if !ok {
			ctx.Error(core.M("mv-inventories.info.zeroarg"))
			return nil
		}
		name = m.worldName(p.World())
	}
	orNA := func(list []string) string {
		if len(list) == 0 {
			return "&7N/A"
		}
		return strings.Join(list, ", ")
	}
	ctx.Info(core.M("mv-inventories.info.world", "{world}", name))
	if w, ok := m.worldNamed(name); ok {
		var groups []string
		for _, g := range m.Groups.ForWorld(m.worldName(w)) {
			groups = append(groups, g.name)
		}
		ctx.Info(core.M("mv-inventories.info.world.info", "{groups}", orNA(groups)))
	} else {
		ctx.Error(core.M("mv-inventories.error.noworldprofile", "{world}", name))
	}
	ctx.Info(core.M("mv-inventories.info.group", "{group}", name))
	g, ok := m.Groups.Get(name)
	if !ok {
		ctx.Error(core.M("mv-inventories.error.nogroup", "{group}", name))
		return nil
	}
	ctx.Send(core.M("mv-inventories.info.group.info", "{worlds}", orNA(g.configWorlds)))
	ctx.Send(core.M("mv-inventories.info.group.applicableworlds", "{worlds}", orNA(g.ApplicableWorlds())))
	ctx.Send(core.M("mv-inventories.info.group.infoshares", "{shares}", g.shares.String()))
	ctx.Send(core.M("mv-inventories.info.group.applicableshares", "{shares}", g.applicableShares.String()))
	return nil
}

// addWorlds adds worlds to a group. MV: AddWorldsCommand
func (m *Module) addWorlds(ctx *core.Ctx) error {
	g, err := m.groupArg(ctx, 0)
	if err != nil {
		return err
	}
	names := strings.Split(strings.ToLower(ctx.Arg(1)), ",")
	if !ctx.Has("--skip-exist-check") {
		for _, n := range names {
			if !m.validWorldName(n) {
				ctx.Error(core.M("mv-inventories.error.noworld", "{world}", n))
				return nil
			}
		}
	}
	joined := strings.Join(names, ", ")
	if !slices.ContainsFunc(names, func(n string) bool { return !slices.Contains(g.configWorlds, n) }) {
		ctx.Error(core.M("mv-inventories.addworld.worldalreadyexists", "{group}", g.name, "{world}", joined))
		return nil
	}
	g.AddWorlds(names, true)
	ctx.Info(core.M("mv-inventories.addworld.worldadded", "{group}", g.name, "{world}", joined))
	m.sendConflicts(ctx.Issuer, false)
	return nil
}

// editShares changes a group's shares, or with disabled its disabled
// shares, and shows them. MV: AddSharesCommand, RemoveSharesCommand,
// AddDisabledSharesCommand, RemoveDisabledSharesCommand
func (m *Module) editShares(edit func(*WorldGroup, *Shares), disabled bool) func(*core.Ctx) error {
	return func(ctx *core.Ctx) error {
		g, err := m.groupArg(ctx, 0)
		if err != nil {
			return err
		}
		sh, err := sharesArg(ctx.Arg(1))
		if err != nil {
			return err
		}
		edit(g, sh)
		m.Groups.Update(g)
		if disabled {
			ctx.Info(core.M("mv-inventories.disabledshares.nowsharing", "{group}", g.name,
				"{shares}", javaList(g.disabled.Strings())))
		} else {
			negative := allShares.Clone()
			negative.RemoveAll(g.shares)
			ctx.Info(core.M("mv-inventories.shares.nowsharing", "{group}", g.name,
				"{shares}", javaList(g.shares.Strings()), "{negativeshares}", javaList(negative.Strings())))
		}
		m.sendConflicts(ctx.Issuer, false)
		return nil
	}
}

// createGroup makes a group. MV: CreateGroupCommand
func (m *Module) createGroup(ctx *core.Ctx) error {
	name := ctx.Arg(0)
	g, ok := m.Groups.NewGroup(name)
	if !ok {
		ctx.Error(core.M("mv-inventories.group.exists", "{group}", name))
		return nil
	}
	if ctx.Arg(1) != "" {
		g.AddWorlds(strings.Split(ctx.Arg(1), ","), false)
	}
	if ctx.Arg(2) != "" {
		sh, err := sharesArg(ctx.Arg(2))
		if err != nil {
			return err
		}
		g.shares.AddAll(sh)
	}
	m.Groups.Update(g)
	ctx.Info(core.M("mv-inventories.group.creationcomplete", "{group}", name))
	ctx.Info(core.M("mv-inventories.info.group.info", "{worlds}", javaList(g.configWorlds)))
	ctx.Info(core.M("mv-inventories.info.group.infoshares", "{shares}", g.shares.String()))
	m.sendConflicts(ctx.Issuer, false)
	return nil
}

// toggle turns an optional share on or off. MV: ToggleCommand
func (m *Module) toggle(ctx *core.Ctx) error {
	var s *Sharable
	for _, x := range allShares.list {
		if x.Name() == ctx.Arg(0) {
			s = x
		}
	}
	if s == nil {
		return core.InvalidArg(core.M("mv-inventories.error.nosharesspecified"))
	}
	if !s.optional {
		ctx.Error(core.M("mv-inventories.toggle.nooptionalshares", "{share}", s.Name()))
		return nil
	}
	active := m.activeOptionalShares().Clone()
	if active.Has(s) {
		active.Remove(s)
		ctx.Info(core.M("mv-inventories.toggle.nownotusingoptional", "{share}", s.Name()))
	} else {
		active.Add(s)
		ctx.Info(core.M("mv-inventories.toggle.nowusingoptional", "{share}", s.Name()))
		if s == LastLocation {
			ctx.Info(core.M("mv-inventories.toggle.lastlocationinfo", "{url}", lastLocationURL))
		}
	}
	if err := m.Config.Set(m.optionalSharesNode, active); err != nil {
		return err
	}
	return m.Config.Save()
}

// config shows or sets a config.yml option. MV: ConfigCommand
func (m *Module) config(ctx *core.Ctx) error {
	name := ctx.Arg(0)
	n, found := m.Config.Nodes().Find(name)
	notFound := core.M("mv-core.config.node.notfound", "{node}", name)
	if len(ctx.Args) < 2 {
		if !found {
			ctx.Error(core.M("mv-inventories.config.showfailed", "{error}", notFound))
			return nil
		}
		ctx.Send(core.M("mv-inventories.config.currentvalue", "{name}", name, "{value}", core.FormatValue(m.Config.Get(n))))
		return nil
	}
	value := ctx.Arg(1)
	err := core.MsgError(notFound)
	if found {
		var v any
		if v, err = core.ParseString(n, value); err == nil {
			if err = m.Config.Set(n, v); err == nil {
				err = m.Config.Save()
			}
		}
	}
	if err != nil {
		ctx.Error(core.M("mv-inventories.config.setfailed", "{name}", name, "{value}", value, "{error}", core.ErrorMessage(err)))
		return nil
	}
	ctx.Send(core.M("mv-inventories.config.set", "{name}", name, "{value}", value))
	return nil
}

// cacheStats shows how the profile caches are used, in the order Java's
// HashMap gives Multiverse-Inventories' three. MV: CacheCommand
func (m *Module) cacheStats(ctx *core.Ctx) error {
	for _, name := range []string{"playerFileCache", "profileCache", "globalProfileCache"} {
		st := m.store.stat(name)
		requests := st.Hits + st.Misses
		hitRate, missRate, penalty := 1.0, 0.0, 0.0
		if requests > 0 {
			hitRate, missRate = float64(st.Hits)/float64(requests), float64(st.Misses)/float64(requests)
		}
		if st.Loads > 0 {
			penalty = float64(st.LoadTime.Nanoseconds()) / float64(st.Loads) / 1e6
		}
		ctx.Send(core.M("mv-inventories.cache.entry", "{cache}", name))
		ctx.Send(core.M("mv-inventories.cache.hitscount", "{count}", st.Hits))
		ctx.Send(core.M("mv-inventories.cache.missescount", "{count}", st.Misses))
		ctx.Send(core.M("mv-inventories.cache.loadscount", "{count}", st.Loads))
		ctx.Send(core.M("mv-inventories.cache.evictions", "{count}", st.Evictions))
		ctx.Send(core.M("mv-inventories.cache.hitrate", "{rate}", core.JavaDouble(hitRate*100)))
		ctx.Send(core.M("mv-inventories.cache.missrate", "{rate}", core.JavaDouble(missRate*100)))
		ctx.Send(core.M("mv-inventories.cache.avgloadpenalty", "{milliseconds}", core.JavaDouble(penalty)))
		ctx.Send(core.M("mv-inventories.cache.separator"))
	}
	return nil
}

// issuerAwarePlayer answers the online player argument i names, else the
// sender. MV: ACF's Player context with resolve=issuerAware
func (m *Module) issuerAwarePlayer(ctx *core.Ctx, i int) (plugin.Player, error) {
	if ctx.Arg(i) == "" {
		if p, ok := ctx.Player(); ok {
			return p, nil
		}
		return nil, core.ErrUsage
	}
	if p, ok := m.Core.FindPlayer(ctx.Sender, ctx.Arg(i)); ok {
		return p, nil
	}
	return nil, core.MsgError(core.Lit("No player matching " + ctx.Arg(i) + " could be found."))
}

// offlinePlayer answers the player called name: online, or known to the
// player names map. MV: ACF's OfflinePlayer context
func (m *Module) offlinePlayer(name string) (PlayerRef, error) {
	if p, ok := m.Core.FindPlayer(m.Server.Console(), name); ok {
		return p, nil
	}
	if id, ok := m.store.names.Named(name); ok {
		return Ref{id, name}, nil
	}
	return nil, core.MsgError(core.Lit("No player matching " + name + " could be found."))
}

// profileTypeArg reads a profile type argument; with game mode profiles
// off it is not given and every profile is SURVIVAL's.
// MV: MVInvCommandContexts.parseProfileType
func (m *Module) profileTypeArg(ctx *core.Ctx, i int) (ProfileType, int, error) {
	if !m.Config.Bool("share-handling.enable-gamemode-share-handling") {
		return Survival, i, nil
	}
	t, ok := ProfileTypeNamed(ctx.Arg(i))
	if !ok {
		return "", i, core.InvalidArg(core.Lit("Invalid profile type: " + ctx.Arg(i)))
	}
	return t, i + 1, nil
}

func profileTypeNames() []string {
	var out []string
	for _, t := range ProfileTypes() {
		out = append(out, strings.ToLower(string(t)))
	}
	return out
}

func (m *Module) completeGive(ctx *core.Ctx, pos int) []string {
	switch pos {
	case 0:
		return m.Core.PlayerNames(ctx.Sender)
	case 1:
		return m.Core.WorldNames("both")
	}
	if m.Config.Bool("share-handling.enable-gamemode-share-handling") {
		pos--
		if pos == 1 {
			return profileTypeNames()
		}
	}
	switch pos {
	case 2:
		var out []string
		for _, it := range registry.Items[1:] {
			out = append(out, strings.TrimPrefix(it, "minecraft:"))
		}
		return out
	case 3:
		out := make([]string, 64)
		for i := range out {
			out[i] = strconv.Itoa(i + 1)
		}
		return out
	}
	return nil
}

// give puts items in a player's inventory in a world: their live
// inventory when they are there in that game mode, else the stored one.
// MV: GiveCommand
func (m *Module) give(ctx *core.Ctx) error {
	p, err := m.offlinePlayer(ctx.Arg(0))
	if err != nil {
		return err
	}
	w, err := ctx.RequireWorld(1, false)
	if err != nil {
		return err
	}
	typ, next, err := m.profileTypeArg(ctx, 2)
	if err != nil {
		return err
	}
	if len(ctx.Args) <= next {
		return core.ErrUsage
	}
	stack, ok := m.parseGiveItem(ctx, strings.Join(ctx.Args[next:], " "))
	if !ok {
		return nil
	}
	m.Debug(2, "Giving player %s item: %s", p.Name(), m.Server.Items().Format(stack))
	if online, ok := m.Server.Player(p.UUID()); ok && m.worldName(online.World()) == w.Name() && m.profileType(online) == typ {
		online.Inventory().Add(stack)
		ctx.Info(core.M("mv-inventories.give.success", "{player}", p.Name(), "{amount}", stack.Count,
			"{item}", "ItemStack{"+strings.ToUpper(itemPath(stack.Item))+" x "+strconv.Itoa(stack.Count)+"}",
			"{world}", w.Name()))
		return nil
	}
	inv, _ := m.ReadShare(p, w.Name(), typ, Inventory).([]plugin.ItemStack)
	m.writeShare(p, w.Name(), typ, Inventory, m.putItem(inv, stack), true)
	if _, online := m.Server.Player(p.UUID()); !online {
		g := m.store.Global(p.UUID(), p.Name())
		g.LoadOnLogin = true
		m.store.SaveGlobal(g)
		m.store.forgetPlayer(p.UUID())
	}
	ctx.Info(core.M("mv-inventories.give.success", "{player}", p.Name(), "{amount}", stack.Count,
		"{item}", displayName(stack.Item), "{world}", w.Name()))
	return nil
}

// parseGiveItem reads "<item>[components] [amount]", telling the sender
// what is wrong. MV: GiveCommand.parseItemFromString
func (m *Module) parseGiveItem(ctx *core.Ctx, s string) (plugin.ItemStack, bool) {
	amount := 1
	if i := strings.LastIndex(s, " "); i >= 0 {
		if n, err := strconv.Atoi(s[i+1:]); err == nil {
			amount, s = n, s[:i]
		}
	}
	if amount < 1 {
		ctx.Error(core.M("mv-inventories.give.invalidamount"))
		return plugin.ItemStack{}, false
	}
	if amount > maxGive {
		ctx.Error(core.M("mv-inventories.give.amounttoolarge", "{amount}", maxGive))
		return plugin.ItemStack{}, false
	}
	name, components, hasComponents := strings.Cut(s, "[")
	item, ok := matchMaterial(name)
	if !ok {
		ctx.Error(core.M("mv-inventories.give.invalidmaterial", "{material}", name))
		return plugin.ItemStack{}, false
	}
	if !hasComponents {
		return plugin.NewStack(item, amount), true
	}
	stack, err := m.Server.Items().Parse(item.Name()+"["+components, amount)
	if err != nil {
		ctx.Error(core.M("mv-inventories.give.failed", "{error}", err))
		return plugin.ItemStack{}, false
	}
	return stack, true
}

// matchMaterial finds an item by name as Bukkit's Material.matchMaterial
// does: any case, with or without "minecraft:", spaces for underscores.
func matchMaterial(name string) (registry.Item, bool) {
	name = strings.TrimPrefix(strings.ToLower(name), "minecraft:")
	name = strings.Join(strings.Fields(name), "_")
	it, ok := registry.ItemByName("minecraft:" + name)
	return it, ok && it != registry.ItemAir
}

func itemPath(it registry.Item) string { return strings.TrimPrefix(it.Name(), "minecraft:") }

// displayName answers an item's English name from its id, "Diamond Sword".
// MV: ItemStack.getI18NDisplayName
func displayName(it registry.Item) string {
	words := strings.Split(itemPath(it), "_")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// putItem fills empty and similar slots of a stored inventory with stack;
// what does not fit is dropped. MV: GiveCommand.putItemInInventory
func (m *Module) putItem(inv []plugin.ItemStack, stack plugin.ItemStack) []plugin.ItemStack {
	if inv == nil {
		inv = make([]plugin.ItemStack, inventorySize)
	}
	items := m.Server.Items()
	max := items.MaxStackSize(stack)
	left := stack.Count
	for i := 0; i < len(inv) && left > 0; i++ {
		switch {
		case inv[i].Empty():
			inv[i] = stack
			inv[i].Count = min(left, max)
			left -= inv[i].Count
		case items.Similar(inv[i], stack):
			n := min(left, max-inv[i].Count)
			inv[i].Count += n
			left -= n
		}
	}
	return inv
}

// bulkMillis writes a duration in milliseconds as "%.4f".
func bulkMillis(d time.Duration) string {
	return fmt.Sprintf("%.4f", float64(d.Nanoseconds())/1e6)
}
