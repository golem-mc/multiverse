package inventories

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// containerKey is a world or group with player files. MV: ContainerKey
type containerKey struct {
	t    ContainerType
	name string
}

// profilesPayload is what a player profile bulk edit acts on.
// MV: PlayerProfilesPayload
type profilesPayload struct {
	players    []PlayerRef
	containers []containerKey
	types      []ProfileType
	// includeWorlds adds the worlds of each group given.
	includeWorlds bool
}

var includeWorldsFlag = &core.Flag{Name: "--include-groups-worlds", Aliases: []string{"-i"}}

// bulkEditCommands are /mvinv bulkedit's. MV: the bulkedit commands
func (m *Module) bulkEditCommands() []*core.Cmd {
	const perm = "multiverse.inventories.bulkedit"
	return []*core.Cmd{{
		Name: "bulkedit globalprofile clear", Permission: perm,
		Syntax: "<players> [--clear-all-player-profiles]", MinArgs: 1, MaxArgs: 1,
		Flags:    []*core.Flag{{Name: "--clear-all-player-profiles", Aliases: []string{"-a"}}},
		Complete: func(ctx *core.Ctx, _ int) []string { return m.suggestPlayers(lastWord(ctx)) },
		Run: func(ctx *core.Ctx) error {
			players := m.playersArg(ctx.Arg(0))
			clearAll := ctx.Has("--clear-all-player-profiles")
			m.bulkSummary(ctx.Issuer, [][2]any{{"Players", refNames(players)}})
			ctx.Core.Commands.Confirm(ctx.Issuer, core.M("mv-inventories.bulkedit.globalprofile.clear.confirmprompt"), func() {
				keys := make([]ProfileKey, len(players))
				for i, p := range players {
					keys[i] = ProfileKey{Player: p.UUID(), PlayerName: p.Name()}
				}
				m.runBulkEdit(ctx.Issuer, keys, func(k ProfileKey) error { return m.deleteGlobal(k, clearAll) },
					func(ProfileKey, plugin.Player) bool { return clearAll }, m.readOnly)
			})
			return nil
		},
	}, {
		Name: "bulkedit globalprofile modify", Permission: perm,
		Syntax: "<property> <value> <players>", MinArgs: 3, MaxArgs: 3,
		Complete: func(ctx *core.Ctx, pos int) []string {
			switch pos {
			case 0:
				return []string{"load-on-login", "last-world"}
			case 2:
				return m.suggestPlayers(lastWord(ctx))
			}
			return nil
		},
		Run: m.modifyGlobals,
	}, {
		Name: "bulkedit playerprofile clear", Permission: perm,
		Syntax: "<players> <groups/worlds> [profile-type] [--include-groups-worlds]", MinArgs: 2, MaxArgs: 3,
		Flags: []*core.Flag{includeWorldsFlag}, Complete: m.completePayload(0),
		Run: func(ctx *core.Ctx) error {
			pl, err := m.payloadArgs(ctx, m.playersArg(ctx.Arg(0)), 1)
			if err != nil {
				return err
			}
			m.bulkSummary(ctx.Issuer, pl.summary())
			ctx.Core.Commands.Confirm(ctx.Issuer, core.M("mv-inventories.bulkedit.playerprofile.clear.confirmprompt"), func() {
				m.runFileEdit(ctx.Issuer, pl, func(k ProfileKey) { m.store.DeleteProfiles(k, pl.types) })
			})
			return nil
		},
	}, {
		Name: "bulkedit playerprofile clone-player", Permission: perm,
		Syntax:  "<from-player> <to-player> <groups/worlds> [profile-types] [--include-groups-worlds]",
		MinArgs: 3, MaxArgs: 4, Flags: []*core.Flag{includeWorldsFlag},
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return refNames(m.store.names.All())
			}
			return m.completePayload(1)(ctx, pos)
		},
		Run: func(ctx *core.Ctx) error {
			from, ok := m.store.names.Named(ctx.Arg(0))
			if !ok {
				return core.InvalidArg(core.Lit("Unknown player name: " + ctx.Arg(0)))
			}
			fromName := ctx.Arg(0)
			to := m.playersArg(ctx.Arg(1))
			pl, err := m.payloadArgs(ctx, to, 2)
			if err != nil {
				return err
			}
			m.bulkSummary(ctx.Issuer, pl.summary())
			toNames := strconv.Itoa(len(to)) + " players"
			if len(to) == 1 {
				toNames = to[0].Name()
			}
			ctx.Core.Commands.Confirm(ctx.Issuer, core.M("mv-inventories.bulkedit.playerprofile.cloneplayer.confirmprompt",
				"{fromplayer}", fromName, "{toplayers}", toNames), func() {
				m.runFileEdit(ctx.Issuer, pl, func(k ProfileKey) {
					m.store.CloneProfiles(ProfileKey{Container: k.Container, Name: k.Name, Player: from, PlayerName: fromName}, k, pl.types)
				})
			})
			return nil
		},
	}, {
		Name: "bulkedit playerprofile clone-world-group", Permission: perm,
		Syntax:  "<players> <from-group/world> <to-groups/worlds> [profile-type]",
		MinArgs: 3, MaxArgs: 4, Flags: []*core.Flag{includeWorldsFlag},
		Complete: func(ctx *core.Ctx, pos int) []string {
			switch pos {
			case 0:
				return m.suggestPlayers(lastWord(ctx))
			case 1:
				return m.containerKeyNames()
			}
			return m.completePayload(1)(ctx, pos-1)
		},
		Run: m.cloneWorldGroup,
	}, {
		Name: "bulkedit playerprofile delete", Permission: perm,
		Syntax:  "<sharable> <players> <groups/worlds> [profile-type] [--include-groups-worlds]",
		MinArgs: 3, MaxArgs: 4, Flags: []*core.Flag{includeWorldsFlag},
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return suggestShares(lastWord(ctx))
			}
			return m.completePayload(1)(ctx, pos)
		},
		Run: m.deleteSharable,
	}, {
		Name: "bulkedit migrate inventory-serialization nbt", Permission: perm,
		Run: m.migrateSerialization(true),
	}, {
		Name: "bulkedit migrate inventory-serialization bukkit", Permission: perm,
		Run: m.migrateSerialization(false),
	}}
}

func refNames(refs []PlayerRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Name()
	}
	return out
}

// playersArg reads a comma-separated list of known player names, or
// "@all"; unknown names are left out. MV: parseGlobalProfileKeyArray
func (m *Module) playersArg(s string) []PlayerRef {
	if s == "@all" {
		return m.store.names.All()
	}
	var out []PlayerRef
	for name := range strings.SplitSeq(s, ",") {
		if id, ok := m.store.names.Named(name); ok {
			out = append(out, Ref{id, name})
		}
	}
	return out
}

// suggestPlayers completes a list of known player names.
// MV: MVInvCommandCompletion.suggestPlayerNames
func (m *Module) suggestPlayers(input string) []string {
	if input == "@all" {
		return nil
	}
	names := refNames(m.store.names.All())
	if !strings.Contains(input, ",") {
		return append(names, "@all")
	}
	return addonToCommaSeparated(input, names)
}

func (m *Module) containerKeyNames() []string {
	var out []string
	for _, t := range []ContainerType{ContainerWorld, ContainerGroup} {
		for _, name := range m.store.ContainerNames(t) {
			out = append(out, strings.ToLower(t.String())+"="+name)
		}
	}
	return out
}

// completePayload completes players at first, then containers and
// profile types.
func (m *Module) completePayload(first int) func(*core.Ctx, int) []string {
	return func(ctx *core.Ctx, pos int) []string {
		switch pos - first {
		case 0:
			return m.suggestPlayers(lastWord(ctx))
		case 1:
			return append(m.containerKeyNames(), "@all")
		case 2:
			input := lastWord(ctx)
			if input == "@all" {
				return nil
			}
			if !strings.Contains(input, ",") {
				return append(profileTypeNames(), "@all")
			}
			return addonToCommaSeparated(input, profileTypeNames())
		}
		return nil
	}
}

func containerTypeNamed(s string) (ContainerType, bool) {
	switch strings.ToUpper(s) {
	case "WORLD":
		return ContainerWorld, true
	case "GROUP":
		return ContainerGroup, true
	}
	return 0, false
}

// containerArg reads "type=name" of a world or group with player files.
// MV: MVInvCommandContexts.parseContainerKey
func (m *Module) containerArg(s string) (containerKey, error) {
	typ, name, ok := strings.Cut(s, "=")
	if !ok {
		return containerKey{}, core.InvalidArg(core.Lit("Invalid world/group format: " + s + ". Expected format: type=name"))
	}
	t, ok := containerTypeNamed(typ)
	if !ok {
		return containerKey{}, core.InvalidArg(core.Lit("Unknown container type: " + typ))
	}
	if !slices.Contains(m.store.ContainerNames(t), name) {
		return containerKey{}, core.InvalidArg(core.Lit("The " + typ + " name " + name + " does not have any data."))
	}
	return containerKey{t, name}, nil
}

// containersArg reads "@all" or "type=name1,name2;type2=name3", leaving out
// those with no player files. MV: parseContainerKeyArray
func (m *Module) containersArg(s string) ([]containerKey, error) {
	var out []containerKey
	if s == "@all" {
		for _, t := range []ContainerType{ContainerWorld, ContainerGroup} {
			for _, name := range m.store.ContainerNames(t) {
				out = append(out, containerKey{t, name})
			}
		}
		return out, nil
	}
	for part := range strings.SplitSeq(s, ";") {
		typ, names, ok := strings.Cut(part, "=")
		if !ok {
			return nil, core.InvalidArg(core.Lit("Invalid worlds/groups format: " + part + ". Expected format: type=name1,name2;type2=name3"))
		}
		t, ok := containerTypeNamed(typ)
		if !ok {
			return nil, core.InvalidArg(core.Lit("Unknown container type: " + typ))
		}
		have := m.store.ContainerNames(t)
		for name := range strings.SplitSeq(names, ",") {
			if slices.Contains(have, name) {
				out = append(out, containerKey{t, name})
			}
		}
	}
	return out, nil
}

// typesArg reads a list of profile types; with none, or none valid, every
// type. MV: MVInvCommandContexts.parseProfileTypeArray
func typesArg(s string) []ProfileType {
	var out []ProfileType
	if s != "@all" {
		for name := range strings.SplitSeq(s, ",") {
			if t, ok := ProfileTypeNamed(name); ok {
				out = append(out, t)
			}
		}
	}
	if len(out) == 0 {
		return ProfileTypes()
	}
	return out
}

// payloadArgs reads the containers at i and the optional profile types
// after them.
func (m *Module) payloadArgs(ctx *core.Ctx, players []PlayerRef, i int) (profilesPayload, error) {
	cs, err := m.containersArg(ctx.Arg(i))
	if err != nil {
		return profilesPayload{}, err
	}
	return profilesPayload{players: players, containers: cs, types: typesArg(ctx.Arg(i + 1)),
		includeWorlds: ctx.Has(includeWorldsFlag.Name)}, nil
}

// summary lists what the payload acts on. MV: PlayerProfilesPayload.getSummary
func (pl profilesPayload) summary() [][2]any {
	var worlds, groups, types []string
	for _, c := range pl.containers {
		if c.t == ContainerWorld {
			worlds = append(worlds, c.name)
		} else {
			groups = append(groups, c.name)
		}
	}
	for _, t := range pl.types {
		types = append(types, strings.ToLower(string(t)))
	}
	return [][2]any{{"Players", refNames(pl.players)}, {"Worlds", worlds}, {"Groups", groups}, {"Profile Types", types}}
}

// fileKeys answers a key per player and container, the worlds of the
// groups added with includeWorlds. MV: PlayerProfilesAggregator
func (m *Module) fileKeys(pl profilesPayload) []ProfileKey {
	containers := slices.Clone(pl.containers)
	if pl.includeWorlds {
		for _, c := range pl.containers {
			if g, ok := m.Groups.Get(c.name); ok && c.t == ContainerGroup {
				for _, w := range g.ApplicableWorlds() {
					if k := (containerKey{ContainerWorld, w}); !slices.Contains(containers, k) {
						containers = append(containers, k)
					}
				}
			}
		}
	}
	var out []ProfileKey
	for _, p := range pl.players {
		for _, c := range containers {
			out = append(out, ProfileKey{Container: c.t, Name: c.name, Player: p.UUID(), PlayerName: p.Name()})
		}
	}
	return out
}

// profileKeys answers fileKeys for each of the payload's profile types.
func (m *Module) profileKeys(pl profilesPayload) []ProfileKey {
	var out []ProfileKey
	for _, k := range m.fileKeys(pl) {
		for _, t := range pl.types {
			k.Type = t
			out = append(out, k)
		}
	}
	return out
}

// bulkSummary shows what a bulk edit will change: up to ten names, else a
// count. MV: BulkEditCommand.outputActionSummary
func (m *Module) bulkSummary(is *core.Issuer, entries [][2]any) {
	is.Send(core.M("mv-inventories.bulkedit.summary"))
	for _, e := range entries {
		names, _ := e[1].([]string)
		var value any = strings.Join(names, ", ")
		if len(names) > 10 {
			value = len(names)
		}
		is.Send(core.M("mv-inventories.bulkedit.summaryentry", "{key}", e[0], "{value}", value))
	}
}

// runBulkEdit acts on each key, then reports and refreshes the online
// players affected; offline players' profiles are forgotten after.
// MV: BulkEditAction.execute, BulkEditCommand.runBulkEditAction
func (m *Module) runBulkEdit(is *core.Issuer, keys []ProfileKey, act func(ProfileKey) error,
	affected func(ProfileKey, plugin.Player) bool, update func(plugin.Player)) {
	is.Send(core.M("mv-inventories.bulkedit.starting"))
	start := time.Now()
	var online []plugin.Player
	ok, failed := 0, 0
	for _, k := range keys {
		if p, on := m.Server.Player(k.Player); on && affected(k, p) && !slices.Contains(online, p) {
			online = append(online, p)
		}
		if err := act(k); err != nil {
			failed++
			m.Log.Error("Bulk edit failed", "player", k.PlayerName, "error", err)
		} else {
			ok++
		}
	}
	for _, k := range keys {
		if _, on := m.Server.Player(k.Player); !on {
			m.store.forgetPlayer(k.Player)
		}
	}
	is.Send(core.M("mv-inventories.bulkedit.successcount", "{count}", ok))
	if failed > 0 {
		is.Error(core.M("mv-inventories.bulkedit.failurecount", "{count}", failed))
	}
	is.Send(core.M("mv-inventories.bulkedit.completedtime", "{milliseconds}", bulkMillis(time.Since(start))))
	for _, p := range online {
		update(p)
	}
}

// runFileEdit edits whole player files, marks the players to be loaded on
// their next join, and reloads online players whose current profiles
// changed. MV: AllSharesPlayerFileAction
func (m *Module) runFileEdit(is *core.Issuer, pl profilesPayload, edit func(ProfileKey)) {
	m.runBulkEdit(is, m.fileKeys(pl), func(k ProfileKey) error {
		edit(k)
		m.loadOnLogin(k)
		return nil
	}, func(k ProfileKey, p plugin.Player) bool {
		if !slices.Contains(pl.types, m.profileType(p)) {
			return false
		}
		world := m.worldName(p.World())
		groups := m.Groups.ForWorld(world)
		unhandled := m.enabledShares()
		for _, g := range groups {
			unhandled.RemoveAll(g.applicableShares)
		}
		if !unhandled.Empty() {
			return k.Container == ContainerWorld && k.Name == world
		}
		return k.Container == ContainerGroup && slices.ContainsFunc(groups, func(g *WorldGroup) bool { return g.name == k.Name })
	}, m.readOnly)
}

// loadOnLogin marks a player's data to be applied when they next join.
func (m *Module) loadOnLogin(k ProfileKey) {
	g := m.store.Global(k.Player, k.PlayerName)
	g.LoadOnLogin = true
	m.store.SaveGlobal(g)
}

// deleteGlobal removes a player's global profile and, with files, every
// player file of theirs. MV: FlatFileProfileDataSource.deleteGlobalProfile
func (m *Module) deleteGlobal(k ProfileKey, files bool) error {
	if _, ok := m.store.ExistingGlobal(k.Player, k.PlayerName); !ok {
		return errors.New("Invalid global profile for player: " + k.PlayerName)
	}
	if err := m.store.DeleteGlobal(k.Player); err != nil {
		return err
	}
	if files {
		for _, t := range []ContainerType{ContainerWorld, ContainerGroup} {
			for _, name := range m.store.ContainerNames(t) {
				m.store.DeleteFile(ProfileKey{Container: t, Name: name, Player: k.Player, PlayerName: k.PlayerName})
			}
		}
	}
	return nil
}

// modifyGlobals sets a global profile property for players.
// MV: bulkedit globalprofile ModifyCommand
func (m *Module) modifyGlobals(ctx *core.Ctx) error {
	property, value := ctx.Arg(0), ctx.Arg(1)
	players := m.playersArg(ctx.Arg(2))
	ctx.Core.Commands.Confirm(ctx.Issuer, core.M("mv-inventories.bulkedit.globalprofile.modify.confirmprompt",
		"{property}", property, "{value}", value, "{count}", len(players)), func() {
		count := 0
		for _, p := range players {
			g := m.store.Global(p.UUID(), p.Name())
			var err error
			switch property {
			case "last-world":
				g.LastWorld = value
			case "last-known-name":
				g.LastKnownName = value
			case "load-on-login":
				var v any
				v, err = core.ParseString(&core.Node{Kind: core.KindBool}, value)
				if err == nil {
					g.LoadOnLogin = v.(bool)
				}
			default:
				err = core.MsgError(core.M("mv-core.config.node.notfound", "{node}", property))
			}
			if err != nil {
				ctx.Error(core.M("mv-inventories.bulkedit.globalprofile.modify.failed", "{property}", property,
					"{profile}", p.Name(), "{error}", core.ErrorMessage(err)))
				continue
			}
			m.store.SaveGlobal(g)
			count++
		}
		ctx.Send(core.M("mv-inventories.bulkedit.globalprofile.modify.success", "{property}", property,
			"{value}", value, "{count}", count))
	})
	return nil
}

// cloneWorldGroup copies players' profiles from one world or group to
// others. MV: CloneWorldGroupCommand
func (m *Module) cloneWorldGroup(ctx *core.Ctx) error {
	players := m.playersArg(ctx.Arg(0))
	from, err := m.containerArg(ctx.Arg(1))
	if err != nil {
		return err
	}
	pl, err := m.payloadArgs(ctx, players, 2)
	if err != nil {
		return err
	}
	if slices.Contains(pl.containers, from) {
		ctx.Error(core.M("mv-inventories.bulkedit.playerprofile.cloneworldgroup.samecontainer",
			"{containertype}", from.t.String(), "{container}", from.name))
		return nil
	}
	m.bulkSummary(ctx.Issuer, pl.summary())
	ctx.Core.Commands.Confirm(ctx.Issuer, core.M("mv-inventories.bulkedit.playerprofile.cloneworldgroup.confirmprompt",
		"{containertype}", from.t.String(), "{container}", from.name), func() {
		m.runFileEdit(ctx.Issuer, pl, func(k ProfileKey) {
			src := k
			src.Container, src.Name = from.t, from.name
			m.store.CloneProfiles(src, k, pl.types)
		})
	})
	return nil
}

// deleteSharable removes one sharable from players' profiles.
// MV: bulkedit playerprofile DeleteCommand, PlayerProfileDeleteAction
func (m *Module) deleteSharable(ctx *core.Ctx) error {
	var s *Sharable
	for _, x := range allShares.list {
		if x.Name() == ctx.Arg(0) {
			s = x
		}
	}
	if s == nil {
		return core.InvalidArg(core.M("mv-inventories.error.nosharesspecified"))
	}
	pl, err := m.payloadArgs(ctx, m.playersArg(ctx.Arg(1)), 2)
	if err != nil {
		return err
	}
	m.bulkSummary(ctx.Issuer, pl.summary())
	ctx.Core.Commands.Confirm(ctx.Issuer, core.M("mv-inventories.bulkedit.playerprofile.delete.confirmprompt",
		"{share}", s.Name()), func() {
		m.runBulkEdit(ctx.Issuer, m.profileKeys(pl), func(k ProfileKey) error {
			prof := m.store.Profile(k)
			prof.Set(s, nil)
			m.store.Save(prof)
			m.loadOnLogin(k)
			return nil
		}, func(k ProfileKey, p plugin.Player) bool {
			if m.profileType(p) != k.Type {
				return false
			}
			world := m.worldName(p.World())
			var groups []*WorldGroup
			for _, g := range m.Groups.ForWorld(world) {
				if g.IsSharing(s) {
					groups = append(groups, g)
				}
			}
			if len(groups) == 0 {
				return k.Container == ContainerWorld && k.Name == world
			}
			return k.Container == ContainerGroup && slices.ContainsFunc(groups, func(g *WorldGroup) bool { return g.name == k.Name })
		}, func(p plugin.Player) {
			d := newProfileData()
			d.Set(s, m.ReadShare(p, m.worldName(p.World()), m.profileType(p), s))
			s.apply(m, s, p, d)
		})
	})
	return nil
}

// migrateSerialization switches use-byte-serialization-for-inventory-data
// and rewrites every profile in the new format.
// MV: MigrateInventorySerializationCommand
func (m *Module) migrateSerialization(nbt bool) func(*core.Ctx) error {
	prompt := "mv-inventories.bulkedit.playerprofile.migrateinventoryserialization.bukkit.confirmprompt"
	if nbt {
		prompt = "mv-inventories.bulkedit.playerprofile.migrateinventoryserialization.nbt.confirmprompt"
	}
	return func(ctx *core.Ctx) error {
		ctx.Core.Commands.Confirm(ctx.Issuer, core.M(prompt), func() {
			n, _ := m.Config.Nodes().Find("use-byte-serialization-for-inventory-data")
			if err := m.Config.Set(n, nbt); err == nil {
				_ = m.Config.Save()
			}
			start := time.Now()
			count := 0
			for _, id := range m.store.GlobalUUIDs() {
				g := m.store.Global(id, "")
				for _, t := range []ContainerType{ContainerWorld, ContainerGroup} {
					for _, name := range m.store.ContainerNames(t) {
						for _, typ := range ProfileTypes() {
							prof := m.store.Profile(ProfileKey{Container: t, Name: name, Type: typ, Player: id, PlayerName: g.LastKnownName})
							if prof.Empty() {
								continue
							}
							count++
							m.store.Save(prof)
						}
					}
				}
				if _, on := m.Server.Player(id); !on {
					m.store.forgetPlayer(id)
				}
			}
			ctx.Send(core.M("mv-inventories.bulkedit.playerprofile.migrateinventoryserialization.updated", "{count}", count))
			ctx.Send(core.M("mv-inventories.bulkedit.playerprofile.migrateinventoryserialization.completed",
				"{milliseconds}", time.Since(start).Milliseconds()))
		})
		return nil
	}
}
