package inventories

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// PersistingProfile is a profile a share handling reads or writes, and the
// shares it reads or writes there. MV: PersistingProfile
type PersistingProfile struct {
	Key    ProfileKey
	Shares *Shares
}

// AffectedProfiles are the profiles one share handling writes the player's
// data to and reads it from. MV: AffectedProfiles
type AffectedProfiles struct {
	Write, Read []PersistingProfile
	toRead      *Shares
}

func newAffected() *AffectedProfiles { return &AffectedProfiles{toRead: &Shares{}} }

func (a *AffectedProfiles) addWrite(k ProfileKey, sh *Shares) {
	a.Write = append(a.Write, PersistingProfile{k, sh})
}

func (a *AffectedProfiles) addRead(k ProfileKey, sh *Shares) {
	a.Read = append(a.Read, PersistingProfile{k, sh})
	a.toRead.AddAll(sh)
}

// ShareHandlingEvent is what every share handling event carries: the
// player and the profiles. Cancelling an event leaves the player's data as
// it is. MV: ShareHandlingEvent
type ShareHandlingEvent struct {
	plugin.Cancellable
	Player   plugin.Player
	Profiles *AffectedProfiles
}

// WorldChangeShareHandlingEvent fires before a player's data is switched
// for a world change. MV: WorldChangeShareHandlingEvent
type WorldChangeShareHandlingEvent struct {
	ShareHandlingEvent
	FromWorld, ToWorld string
}

// GameModeChangeShareHandlingEvent fires before a player's data is
// switched for a game mode change. MV: GameModeChangeShareHandlingEvent
type GameModeChangeShareHandlingEvent struct {
	ShareHandlingEvent
	FromGameMode, ToGameMode plugin.GameMode
}

// ReadOnlyShareHandlingEvent fires before a player's data is loaded on join
// with apply-playerdata-on-join. MV: ReadOnlyShareHandlingEvent
type ReadOnlyShareHandlingEvent struct{ ShareHandlingEvent }

// WriteOnlyShareHandlingEvent fires before a player's data is saved as they
// leave. MV: WriteOnlyShareHandlingEvent
type WriteOnlyShareHandlingEvent struct{ ShareHandlingEvent }

var shareEvents = []any{(*WorldChangeShareHandlingEvent)(nil), (*GameModeChangeShareHandlingEvent)(nil),
	(*ReadOnlyShareHandlingEvent)(nil), (*WriteOnlyShareHandlingEvent)(nil)}

// shareEvent is a share handling event with its common part.
type shareEvent interface {
	plugin.Event
	Cancelled() bool
}

// profileType answers the profile type a player's data is kept under: their
// game mode's with enable-gamemode-share-handling, else SURVIVAL.
// MV: ProfileTypes.forPlayer
func (m *Module) profileType(p plugin.Player) ProfileType {
	if m.Config.Bool("share-handling.enable-gamemode-share-handling") {
		return ProfileTypeOf(p.GameMode())
	}
	return Survival
}

// key answers a player's profile key in a container.
func key(t ContainerType, name string, typ ProfileType, p PlayerRef) ProfileKey {
	return ProfileKey{Container: t, Name: name, Type: typ, Player: p.UUID(), PlayerName: p.Name()}
}

// enabledShares answers the standard sharables and the active optional
// ones. MV: Sharables.enabled
func (m *Module) enabledShares() *Shares {
	sh := standardShares.Clone()
	sh.AddAll(m.activeOptionalShares())
	return sh
}

// ungroupedShares answers the shares a world in no group uses.
func (m *Module) ungroupedShares(groups []*WorldGroup) *Shares {
	if len(groups) == 0 && !m.Config.Bool("share-handling.use-optionals-for-ungrouped-worlds") {
		return standardShares.Clone()
	}
	return m.enabledShares()
}

// handle runs a share handling: fires ev, then reads the player's data into
// a snapshot, applies the read profiles to the player and writes the
// snapshot into the write profiles. MV: ShareHandler.handleSharing
func (m *Module) handle(p plugin.Player, a *AffectedProfiles, ev shareEvent) {
	m.Server.Events().Fire(ev)
	if ev.Cancelled() {
		m.Debug(1, "Share handling has been cancelled by another plugin!")
		return
	}
	m.Debug(2, "Change affected by %d fromProfiles and %d toProfiles", len(a.Write), len(a.Read))
	snapshot := newProfileData()
	for _, s := range m.enabledShares().list {
		s.read(m, s, p, snapshot)
	}
	m.handling[p.UUID()] = a
	for _, rp := range a.Read {
		m.applyProfile(p, rp)
	}
	for _, wp := range a.Write {
		if wp.Shares.Empty() {
			continue
		}
		prof := m.store.Profile(wp.Key)
		prof.update(snapshot, wp.Shares)
		m.store.Save(prof)
		m.Debug(2, "Persisted: %s to %s:%s (%s) for player %s", wp.Shares, wp.Key.Container, wp.Key.Name, wp.Key.Type, wp.Key.PlayerName)
	}
	delete(m.handling, p.UUID())
}

// applyProfile sets a player from one profile's shares.
// MV: ShareHandlingUpdater.updatePlayer
func (m *Module) applyProfile(p plugin.Player, rp PersistingProfile) {
	if rp.Shares.Empty() {
		return
	}
	p.CloseInventory()
	prof := m.store.Profile(rp.Key)
	var loaded, defaulted []string
	for _, s := range rp.Shares.list {
		if s.apply(m, s, p, &prof.ProfileData) {
			loaded = append(loaded, s.Name())
		} else {
			defaulted = append(defaulted, s.Name())
		}
	}
	where := rp.Key.Container.String() + ":" + rp.Key.Name + " (" + string(rp.Key.Type) + ")"
	if len(loaded) > 0 {
		m.Debug(2, "Updated: [%s] for %s for %s", strings.Join(loaded, ", "), rp.Key.PlayerName, where)
	}
	if len(defaulted) > 0 {
		m.Debug(2, "Defaulted: [%s] for %s for %s", strings.Join(defaulted, ", "), rp.Key.PlayerName, where)
	}
}

// IsHandlingSharable reports whether a share handling is setting s on the
// player right now. MV: PlayerShareHandlingState.isHandlingSharable
func (m *Module) IsHandlingSharable(p plugin.Player, s *Sharable) bool {
	a, ok := m.handling[p.UUID()]
	return ok && a.toRead.Has(s)
}

// worldChange switches a player's data from one world's to another's.
// MV: WorldChangeShareHandler
func (m *Module) worldChange(p plugin.Player, from, to string) {
	m.Debug(1, "=== %s traveling from world: %s to world: %s ===", p.Name(), from, to)
	a := newAffected()
	fromGroups, toGroups := m.Groups.ForWorld(from), m.Groups.ForWorld(to)
	typ := m.profileType(p)
	worldKey := func(w string) ProfileKey { return key(ContainerWorld, w, typ, p) }
	switch {
	case !m.hasBypass(p, bypassWorld, from):
		// The groups left write their shares; the world left the rest.
		handled := &Shares{}
		for _, g := range fromGroups {
			if !g.ContainsWorld(to) {
				a.addWrite(key(ContainerGroup, g.name, typ, p), g.applicableShares)
			}
			handled.AddAll(g.applicableShares)
			handled.AddAll(g.disabled)
		}
		write := m.enabledShares()
		if !m.Config.Bool("performance.always-write-world-profile") {
			write.RemoveAll(handled)
		}
		if !write.Empty() {
			a.addWrite(worldKey(from), write)
		}
		// The groups entered read their shares; the world entered the rest.
		handled = &Shares{}
		allTeleports := m.Config.Bool("sharables.apply-last-location-for-all-teleports")
		for _, g := range toGroups {
			if m.hasBypass(p, bypassGroup, g.name) {
				continue
			}
			if m.fromWorldNotIn(g, from) {
				read := g.applicableShares.Clone()
				if !allTeleports {
					read.Remove(LastLocation)
				}
				a.addRead(key(ContainerGroup, g.name, typ, p), read)
			}
			handled.AddAll(g.applicableShares)
			handled.AddAll(g.disabled)
		}
		unhandled := m.ungroupedShares(toGroups)
		unhandled.RemoveAll(handled)
		if !allTeleports {
			unhandled.Remove(LastLocation)
		}
		if !unhandled.Empty() {
			a.addRead(worldKey(to), unhandled)
		}
	case m.Config.Bool("performance.always-write-world-profile"):
		// Keep the world's profile current in case the bypass is removed.
		a.addWrite(worldKey(from), m.ungroupedShares(fromGroups))
	}
	m.handle(p, a, &WorldChangeShareHandlingEvent{ShareHandlingEvent{Player: p, Profiles: a}, from, to})
}

// fromWorldNotIn reports whether a player coming from a world enters g
// anew. MV: isFromWorldNotInToWorldGroup
func (m *Module) fromWorldNotIn(g *WorldGroup, from string) bool {
	if m.Config.Bool("share-handling.default-ungrouped-worlds") && !m.Groups.HasConfiguredGroup(from) &&
		g == m.Groups.Default() {
		return false
	}
	return !g.ContainsWorld(from)
}

// gameModeChange switches a player's data from one game mode's profiles to
// another's. MV: GameModeShareHandler
func (m *Module) gameModeChange(p plugin.Player, from, to plugin.GameMode) {
	fromType, toType := ProfileTypeOf(from), ProfileTypeOf(to)
	world := m.worldName(p.World())
	m.Debug(1, "=== %s changing game mode from: ProfileType:%s to: ProfileType:%s for world: %s ===", p.Name(), fromType, toType, world)
	a := newAffected()
	groups := m.Groups.ForWorld(world)
	switch {
	case !m.hasBypass(p, bypassWorld, world) && !m.hasBypass(p, bypassGameMode, strings.ToLower(gameModeName(to))):
		handled := &Shares{}
		for _, g := range groups {
			a.addWrite(key(ContainerGroup, g.name, fromType, p), g.applicableShares)
			a.addRead(key(ContainerGroup, g.name, toType, p), g.applicableShares)
			handled.AddAll(g.applicableShares)
			handled.AddAll(g.disabled)
		}
		unhandled := m.ungroupedShares(groups)
		unhandled.RemoveAll(handled)
		if !unhandled.Empty() {
			a.addRead(key(ContainerWorld, world, toType, p), unhandled)
		}
		switch {
		case m.Config.Bool("performance.always-write-world-profile"):
			write := standardShares.Clone()
			if m.Config.Bool("share-handling.use-optionals-for-ungrouped-worlds") {
				write = m.enabledShares()
			}
			a.addWrite(key(ContainerWorld, world, fromType, p), write)
		case !unhandled.Empty():
			a.addWrite(key(ContainerWorld, world, fromType, p), unhandled)
		}
	case m.Config.Bool("performance.always-write-world-profile"):
		a.addWrite(key(ContainerWorld, world, fromType, p), m.ungroupedShares(groups))
	}
	m.handle(p, a, &GameModeChangeShareHandlingEvent{ShareHandlingEvent{Player: p, Profiles: a}, from, to})
}

// gameModeName answers Bukkit's GameMode name.
func gameModeName(gm plugin.GameMode) string { return string(ProfileTypeOf(gm)) }

// readOnly loads a player's data from their world's and groups' profiles.
// MV: ReadOnlyShareHandler
func (m *Module) readOnly(p plugin.Player) {
	a := newAffected()
	world := m.worldName(p.World())
	typ := m.profileType(p)
	unhandled := m.enabledShares()
	for _, g := range m.Groups.ForWorld(world) {
		a.addRead(key(ContainerGroup, g.name, typ, p), g.applicableShares)
		unhandled.RemoveAll(g.applicableShares)
		unhandled.RemoveAll(g.disabled)
	}
	if !unhandled.Empty() {
		a.addRead(key(ContainerWorld, world, typ, p), unhandled)
	}
	m.handle(p, a, &ReadOnlyShareHandlingEvent{ShareHandlingEvent{Player: p, Profiles: a}})
}

// writeOnly saves a player's data into their world's and groups' profiles.
// MV: WriteOnlyShareHandler
func (m *Module) writeOnly(p plugin.Player) {
	a := newAffected()
	world := m.worldName(p.World())
	typ := m.profileType(p)
	unhandled := m.enabledShares()
	for _, g := range m.Groups.ForWorld(world) {
		a.addWrite(key(ContainerGroup, g.name, typ, p), g.applicableShares)
		unhandled.RemoveAll(g.applicableShares)
		unhandled.RemoveAll(g.disabled)
	}
	write := unhandled
	if m.Config.Bool("performance.always-write-world-profile") {
		write = m.enabledShares()
	}
	if !write.Empty() {
		a.addWrite(key(ContainerWorld, world, typ, p), write)
	}
	m.handle(p, a, &WriteOnlyShareHandlingEvent{ShareHandlingEvent{Player: p, Profiles: a}})
}

// writeShare stores one sharable's value in the profiles of a player's
// world and of its groups that share it, saving them when save is set.
// MV: SingleShareWriter
func (m *Module) writeShare(p PlayerRef, world string, typ ProfileType, s *Sharable, v any, save bool) {
	if s.optional && !m.activeOptionalShares().Has(s) {
		m.Debug(2, "Skipping write for optional share: %s", s)
		return
	}
	write := func(k ProfileKey) {
		prof := m.store.Profile(k)
		if equalValue(prof.Get(s), v) {
			return
		}
		prof.Set(s, v)
		if save {
			m.store.Save(prof)
		}
	}
	write(key(ContainerWorld, world, typ, p))
	for _, g := range m.Groups.ForWorld(world) {
		if g.applicableShares.Has(s) {
			write(key(ContainerGroup, g.name, typ, p))
		}
	}
}

func equalValue(a, b any) bool {
	la, okA := a.(*Loc)
	lb, okB := b.(*Loc)
	if okA || okB {
		return okA && okB && (la == nil) == (lb == nil) && (la == nil || *la == *lb)
	}
	return a == nil && b == nil
}

// ReadShare answers one sharable's value for a player in a world: from the
// first of its groups that shares it, else the world's profile.
// MV: SingleShareReader
func (m *Module) ReadShare(p PlayerRef, world string, typ ProfileType, s *Sharable) any {
	for _, g := range m.Groups.ForWorld(world) {
		if g.IsSharing(s) {
			return m.store.Profile(key(ContainerGroup, g.name, typ, p)).Get(s)
		}
	}
	return m.store.Profile(key(ContainerWorld, world, typ, p)).Get(s)
}

// writeLastLocation records where a player is leaving from.
func (m *Module) writeLastLocation(p plugin.Player, loc plugin.Location) {
	m.writeShare(p, m.worldName(p.World()), m.profileType(p), LastLocation, m.locOf(loc, ""), false)
}
