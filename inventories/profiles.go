package inventories

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/google/uuid"
)

// ContainerType is what a profile belongs to: a world or a group.
// MV: ContainerType
type ContainerType uint8

const (
	ContainerWorld ContainerType = iota
	ContainerGroup
)

func (t ContainerType) String() string {
	if t == ContainerGroup {
		return "GROUP"
	}
	return "WORLD"
}

// folder answers the data folder the container type's profiles are in.
func (t ContainerType) folder() string {
	if t == ContainerGroup {
		return "groups"
	}
	return "worlds"
}

// ProfileType is the game mode a profile is for: SURVIVAL, CREATIVE,
// ADVENTURE or SPECTATOR. MV: ProfileType, ProfileTypes
type ProfileType string

const (
	Survival  ProfileType = "SURVIVAL"
	Creative  ProfileType = "CREATIVE"
	Adventure ProfileType = "ADVENTURE"
	Spectator ProfileType = "SPECTATOR"
)

// ProfileTypes answers every profile type.
func ProfileTypes() []ProfileType { return []ProfileType{Survival, Creative, Adventure, Spectator} }

// ProfileTypeOf answers the profile type of a game mode.
func ProfileTypeOf(gm plugin.GameMode) ProfileType {
	switch gm {
	case plugin.Creative:
		return Creative
	case plugin.Adventure:
		return Adventure
	case plugin.Spectator:
		return Spectator
	}
	return Survival
}

// ProfileTypeNamed answers the profile type called name, ignoring case.
func ProfileTypeNamed(name string) (ProfileType, bool) {
	for _, t := range ProfileTypes() {
		if strings.EqualFold(string(t), name) {
			return t, true
		}
	}
	return "", false
}

// PlayerRef is a player whose profiles are read or written, online or
// not: a plugin.Player, plugin.OfflinePlayer or Ref. MV: GlobalProfileKey
type PlayerRef interface {
	UUID() plugin.UUID
	Name() string
}

// Ref is a PlayerRef by UUID and last known name.
type Ref struct {
	ID         plugin.UUID
	PlayerName string
}

func (r Ref) UUID() plugin.UUID { return r.ID }
func (r Ref) Name() string      { return r.PlayerName }

// ProfileKey names one profile: a player's data for one game mode in one
// world or group. Profiles are the same when the container, type and
// player's UUID are. MV: ProfileKey
type ProfileKey struct {
	Container ContainerType
	// Name is the world's or group's name.
	Name   string
	Type   ProfileType
	Player plugin.UUID
	// PlayerName names the player's file.
	PlayerName string
}

type profileID struct {
	container ContainerType
	name      string
	typ       ProfileType
	player    plugin.UUID
}

func (k ProfileKey) id() profileID { return profileID{k.Container, k.Name, k.Type, k.Player} }

// fileID names a player file: every profile type of a player in a container.
type fileID struct {
	container ContainerType
	name      string
	player    plugin.UUID
}

func (k ProfileKey) file() fileID { return fileID{k.Container, k.Name, k.Player} }

// ProfileData is a set of sharables' values. MV: ProfileDataSnapshot
type ProfileData struct{ data map[*Sharable]any }

func newProfileData() *ProfileData { return &ProfileData{data: map[*Sharable]any{}} }

// Get answers the value of s, nil when there is none.
func (d *ProfileData) Get(s *Sharable) any { return d.data[s] }

// Set stores v for s; nil removes it.
func (d *ProfileData) Set(s *Sharable, v any) {
	if v == nil {
		delete(d.data, s)
		return
	}
	d.data[s] = v
}

// Empty reports whether no value is stored.
func (d *ProfileData) Empty() bool { return len(d.data) == 0 }

// update copies the values snapshot has for shares. MV: update(snapshot, shares)
func (d *ProfileData) update(snapshot *ProfileData, shares *Shares) {
	for _, s := range shares.list {
		if v, ok := snapshot.data[s]; ok {
			d.data[s] = v
		}
	}
}

// PlayerProfile is one profile's data. MV: PlayerProfile
type PlayerProfile struct {
	ProfileData
	Key ProfileKey
}

// GlobalProfile is what is kept about a player across worlds: players/<uuid>.json.
// MV: GlobalProfile
type GlobalProfile struct {
	UUID          plugin.UUID
	LastWorld     string
	LastKnownName string
	LoadOnLogin   bool
}

// store reads and writes the profile files, keeping what it read: the
// worlds/<world>/<player>.json and groups/<group>/<player>.json files, one
// section per profile type, and players/<uuid>.json.
// MV: FlatFileProfileDataSource, ProfileFilesLocator, ProfileCacheManager
type store struct {
	m        *Module
	dir      string
	files    map[fileID]map[string]any
	profiles map[profileID]*PlayerProfile
	globals  map[plugin.UUID]*GlobalProfile
	names    *playerNames
	// stats count each cache's use for /mvinv cache stats, by its name in
	// Multiverse-Inventories: playerFileCache, profileCache, globalProfileCache.
	stats map[string]*CacheStats
}

// CacheStats counts a cache's use. MV: Caffeine's CacheStats
type CacheStats struct {
	Hits, Misses, Loads, Evictions int
	LoadTime                       time.Duration
}

func (s *store) stat(name string) *CacheStats {
	if s.stats == nil {
		s.stats = map[string]*CacheStats{}
	}
	if s.stats[name] == nil {
		s.stats[name] = &CacheStats{}
	}
	return s.stats[name]
}

// loaded counts a miss and its load, which began at start.
func (s *store) loaded(name string, start time.Time) {
	st := s.stat(name)
	st.Misses++
	st.Loads++
	st.LoadTime += time.Since(start)
}

func newStore(m *Module, dir string) *store {
	s := &store{m: m, dir: dir}
	s.clear()
	for _, sub := range []string{"worlds", "groups", "players"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			m.Log.Error("Could not create a data folder", "folder", sub, "error", err)
		}
	}
	s.names = newPlayerNames(s)
	return s
}

// clear forgets what was read. MV: ProfileCacheManager.clearAllCache
func (s *store) clear() {
	if s.files != nil {
		s.stat("playerFileCache").Evictions += len(s.files)
		s.stat("profileCache").Evictions += len(s.profiles)
		s.stat("globalProfileCache").Evictions += len(s.globals)
	}
	s.files = map[fileID]map[string]any{}
	s.profiles = map[profileID]*PlayerProfile{}
	s.globals = map[plugin.UUID]*GlobalProfile{}
}

// forgetPlayer drops a player's profiles from memory, as they leave.
func (s *store) forgetPlayer(id plugin.UUID) {
	for k := range s.files {
		if k.player == id {
			delete(s.files, k)
			s.stat("playerFileCache").Evictions++
		}
	}
	for k := range s.profiles {
		if k.player == id {
			delete(s.profiles, k)
			s.stat("profileCache").Evictions++
		}
	}
	if _, ok := s.globals[id]; ok {
		delete(s.globals, id)
		s.stat("globalProfileCache").Evictions++
	}
}

// forgetContainer drops a world's or group's profiles from memory.
// MV: ProfileContainer.clearContainerCache
func (s *store) forgetContainer(t ContainerType, name string) {
	for k := range s.files {
		if k.container == t && k.name == name {
			delete(s.files, k)
			s.stat("playerFileCache").Evictions++
		}
	}
	for k := range s.profiles {
		if k.container == t && k.name == name {
			delete(s.profiles, k)
			s.stat("profileCache").Evictions++
		}
	}
}

// containerDir answers the folder of a world's or group's player files.
func (s *store) containerDir(t ContainerType, name string) string {
	return filepath.Join(s.dir, t.folder(), name)
}

// playerFile answers the file of a player's profiles in a container.
// MV: ProfileFilesLocator.getPlayerProfileFile
func (s *store) playerFile(t ContainerType, name, player string) string {
	return filepath.Join(s.containerDir(t, name), player+".json")
}

// fileData answers a player file's sections, reading it once.
func (s *store) fileData(k ProfileKey) map[string]any {
	if data, ok := s.files[k.file()]; ok {
		s.stat("playerFileCache").Hits++
		return data
	}
	defer s.loaded("playerFileCache", time.Now())
	data := map[string]any{}
	path := s.playerFile(k.Container, k.Name, k.PlayerName)
	if raw, err := os.ReadFile(path); err == nil {
		if m, err := decodeJSON(raw); err == nil {
			data = m
		} else {
			s.m.Log.Error("Could not load file", "file", path, "error", err)
		}
	}
	s.files[k.file()] = data
	return data
}

// Profile answers a profile, read from its file the first time.
// MV: ProfileDataSource.getPlayerProfile
func (s *store) Profile(k ProfileKey) *PlayerProfile {
	if p, ok := s.profiles[k.id()]; ok {
		s.stat("profileCache").Hits++
		return p
	}
	defer s.loaded("profileCache", time.Now())
	p := &PlayerProfile{ProfileData: *newProfileData(), Key: k}
	if section, ok := s.fileData(k)[string(k.Type)].(map[string]any); ok {
		s.decodeProfile(p, section)
	}
	s.profiles[k.id()] = p
	return p
}

// decodeProfile reads a profile section. MV: PlayerProfileJsonSerializer.deserialize
func (s *store) decodeProfile(p *PlayerProfile, section map[string]any) {
	for key, raw := range section {
		if raw == nil {
			continue
		}
		if strings.EqualFold(key, "stats") {
			stats, ok := raw.(map[string]any)
			if str, isStr := raw.(string); isStr {
				if str == "" {
					continue
				}
				parsed, err := decodeJSON([]byte(str))
				stats, ok = parsed, err == nil
			}
			if !ok {
				s.m.Log.Warn("Could not parse stats", "player", p.Key.PlayerName)
				continue
			}
			for tag, v := range stats {
				sh, ok := sharableByTag(true, tag)
				if !ok {
					s.m.Log.Warn("Could not parse stat", "stat", tag, "player", p.Key.PlayerName)
					continue
				}
				if val, ok := sh.decode(s.m, v); ok {
					p.Set(sh, val)
				}
			}
			continue
		}
		sh, ok := sharableByTag(false, key)
		if !ok {
			s.m.Debug(1, "Player fileTag '%s' is unrecognized!", key)
			continue
		}
		if val, ok := sh.decode(s.m, raw); ok {
			p.Set(sh, val)
		}
	}
}

// encodeProfile writes a profile's section: each value under its file tag
// in the sharables' order, the stats last. MV: PlayerProfileJsonSerializer.serialize
func (s *store) encodeProfile(p *PlayerProfile) obj {
	out, stats := obj{}, obj{}
	for _, sh := range allShares.list {
		v, ok := p.data[sh]
		if !ok {
			continue
		}
		enc := sh.encode(s.m, v)
		if enc == nil {
			continue
		}
		if sh.stat {
			stats = append(stats, kv{sh.fileTag, enc})
		} else {
			out = append(out, kv{sh.fileTag, enc})
		}
	}
	if len(stats) > 0 {
		out = append(out, kv{"stats", stats})
	}
	return out
}

// Save writes a profile into its player file; an emptied profile's section
// is removed. MV: updatePlayerProfile
func (s *store) Save(p *PlayerProfile) {
	section := s.encodeProfile(p)
	data := s.fileData(p.Key)
	if len(section) == 0 {
		if _, ok := data[string(p.Key.Type)]; !ok {
			return
		}
		delete(data, string(p.Key.Type))
	} else {
		data[string(p.Key.Type)] = section
	}
	s.writeFile(s.playerFile(p.Key.Container, p.Key.Name, p.Key.PlayerName), data)
}

// writeFile writes a player file's sections, in profile type order.
func (s *store) writeFile(path string, data map[string]any) {
	out := obj{}
	for _, t := range ProfileTypes() {
		if v, ok := data[string(t)]; ok {
			out = append(out, kv{string(t), v})
		}
	}
	for k, v := range data {
		if _, ok := ProfileTypeNamed(k); !ok {
			out = append(out, kv{k, v})
		}
	}
	s.write(path, out)
}

func (s *store) write(path string, v any) {
	raw, err := marshalJSON(v)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	if err == nil {
		err = os.WriteFile(path, raw, 0o644)
	}
	if err != nil {
		s.m.Log.Error("Could not save data", "file", path, "error", err)
	}
}

// DeleteProfile clears one profile type from a player file.
// MV: deletePlayerProfile
func (s *store) DeleteProfile(k ProfileKey) {
	if p, ok := s.profiles[k.id()]; ok {
		clear(p.data)
	}
	path := s.playerFile(k.Container, k.Name, k.PlayerName)
	if _, err := os.Stat(path); err != nil {
		return
	}
	data := s.fileData(k)
	delete(data, string(k.Type))
	s.writeFile(path, data)
}

// DeleteFile removes a player's file in a container, every profile type.
// MV: deletePlayerFile
func (s *store) DeleteFile(k ProfileKey) {
	for _, t := range ProfileTypes() {
		k.Type = t
		if p, ok := s.profiles[k.id()]; ok {
			clear(p.data)
		}
	}
	delete(s.files, k.file())
	path := s.playerFile(k.Container, k.Name, k.PlayerName)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.m.Log.Warn("Could not delete file", "file", path, "error", err)
	}
}

// DeleteProfiles clears profile types from a player file, removing the file
// when they are all of them. MV: deletePlayerProfiles
func (s *store) DeleteProfiles(k ProfileKey, types []ProfileType) {
	if allTypes(types) {
		s.DeleteFile(k)
		return
	}
	for _, t := range types {
		k.Type = t
		s.DeleteProfile(k)
	}
}

// allTypes reports whether types holds every profile type.
func allTypes(types []ProfileType) bool {
	return !slices.ContainsFunc(ProfileTypes(), func(t ProfileType) bool { return !slices.Contains(types, t) })
}

// forgetFile forgets what was read of a player file.
func (s *store) forgetFile(k ProfileKey) {
	delete(s.files, k.file())
	for _, t := range ProfileTypes() {
		k.Type = t
		delete(s.profiles, k.id())
	}
}

// CloneProfiles copies profile types from one player file to another, a
// type the source lacks being removed from the target; with every type, the
// file itself. MV: clonePlayerProfiles
func (s *store) CloneProfiles(from, to ProfileKey, types []ProfileType) {
	fromPath, toPath := s.playerFile(from.Container, from.Name, from.PlayerName), s.playerFile(to.Container, to.Name, to.PlayerName)
	if allTypes(types) {
		raw, err := os.ReadFile(fromPath)
		if err != nil {
			return
		}
		s.forgetFile(to)
		err = os.MkdirAll(filepath.Dir(toPath), 0o755)
		if err == nil {
			err = os.WriteFile(toPath, raw, 0o644)
		}
		if err != nil {
			s.m.Log.Error("Could not clone file", "from", fromPath, "to", toPath, "error", err)
		}
		return
	}
	s.forgetFile(to)
	src, dst := s.fileData(from), s.fileData(to)
	for _, t := range types {
		if section, ok := src[string(t)].(map[string]any); ok && len(section) > 0 {
			dst[string(t)] = section
		} else {
			delete(dst, string(t))
		}
	}
	s.writeFile(toPath, dst)
}

// migrateName renames a player's files after a name change.
// MV: FlatFileProfileDataSource.migratePlayerProfileName
func (s *store) migrateName(id plugin.UUID, oldName, newName string) {
	s.forgetPlayer(id)
	for _, t := range []ContainerType{ContainerWorld, ContainerGroup} {
		for _, container := range s.ContainerNames(t) {
			from, to := s.playerFile(t, container, oldName), s.playerFile(t, container, newName)
			if _, err := os.Stat(from); err != nil {
				continue
			}
			if _, err := os.Stat(to); err == nil {
				s.m.Log.Warn("Data already exists for player. Not migrating.", "player", newName, "type", t.String(), "name", container)
				continue
			}
			if err := os.Rename(from, to); err != nil {
				s.m.Log.Warn("Could not rename old data file", "player", oldName, "error", err)
			}
		}
	}
}

// ContainerNames answers the worlds or groups that have player files.
// MV: listContainerDataNames
func (s *store) ContainerNames(t ContainerType) []string {
	entries, _ := os.ReadDir(filepath.Join(s.dir, t.folder()))
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// PlayerNames answers the players with a file in a world or group.
// MV: listPlayerProfileNames
func (s *store) PlayerNames(t ContainerType, name string) []string {
	entries, _ := os.ReadDir(s.containerDir(t, name))
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		}
	}
	return out
}

func (s *store) globalFile(id string) string {
	return filepath.Join(s.dir, "players", id+".json")
}

// Global answers a player's global profile, read the first time; a
// players/<name>.json of Multiverse-Inventories 2 is renamed to the UUID.
// MV: getGlobalProfile
func (s *store) Global(id plugin.UUID, name string) *GlobalProfile {
	if g, ok := s.globals[id]; ok {
		s.stat("globalProfileCache").Hits++
		return g
	}
	defer s.loaded("globalProfileCache", time.Now())
	path := s.globalFile(id.String())
	if name != "" {
		if legacy := s.globalFile(name); legacy != path {
			if _, err := os.Stat(legacy); err == nil {
				if err := os.Rename(legacy, path); err != nil {
					s.m.Log.Warn("Could not properly migrate player global data file", "player", name)
				}
			}
		}
	}
	g := &GlobalProfile{UUID: id, LastKnownName: name}
	if raw, err := os.ReadFile(path); err == nil {
		if m, err := decodeJSON(raw); err == nil {
			data, _ := m["playerData"].(map[string]any)
			g.LastWorld, _ = data["lastWorld"].(string)
			if n, ok := data["lastKnownName"].(string); ok {
				g.LastKnownName = n
			}
			g.LoadOnLogin, _ = data["loadOnLogin"].(bool)
		}
	}
	s.globals[id] = g
	return g
}

// ExistingGlobal answers a player's global profile when its file exists.
func (s *store) ExistingGlobal(id plugin.UUID, name string) (*GlobalProfile, bool) {
	if g, ok := s.globals[id]; ok {
		return g, true
	}
	if _, err := os.Stat(s.globalFile(id.String())); err != nil {
		return nil, false
	}
	return s.Global(id, name), true
}

// SaveGlobal writes a global profile. MV: updateGlobalProfile
func (s *store) SaveGlobal(g *GlobalProfile) {
	s.write(s.globalFile(g.UUID.String()), obj{{"playerData", obj{
		{"lastWorld", g.LastWorld}, {"lastKnownName", g.LastKnownName}, {"loadOnLogin", g.LoadOnLogin}}}})
	if s.names.set(g.UUID, g.LastKnownName) {
		s.names.save()
	}
}

// DeleteGlobal removes a player's global profile file.
func (s *store) DeleteGlobal(id plugin.UUID) error {
	delete(s.globals, id)
	return os.Remove(s.globalFile(id.String()))
}

// GlobalUUIDs answers the players with a global profile.
func (s *store) GlobalUUIDs() []plugin.UUID {
	entries, _ := os.ReadDir(filepath.Join(s.dir, "players"))
	var out []plugin.UUID
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, err := uuid.Parse(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			s.m.Log.Warn("File name is not a valid UUID", "file", e.Name())
			continue
		}
		out = append(out, id)
	}
	return out
}

// playerNames is playernames.json: each player's last known name by UUID.
// MV: PlayerNamesMapper
type playerNames struct {
	s      *store
	byID   map[plugin.UUID]string
	byName map[string]plugin.UUID
}

func newPlayerNames(s *store) *playerNames {
	return &playerNames{s: s, byID: map[plugin.UUID]string{}, byName: map[string]plugin.UUID{}}
}

func (n *playerNames) path() string { return filepath.Join(n.s.dir, "playernames.json") }

// load reads playernames.json, or builds it from the global profiles.
// MV: PlayerNamesMapper.loadMap
func (n *playerNames) load() {
	n.byID, n.byName = map[plugin.UUID]string{}, map[string]plugin.UUID{}
	if raw, err := os.ReadFile(n.path()); err == nil {
		if m, err := decodeJSON(raw); err == nil && len(m) > 0 {
			for k, v := range m {
				id, err := uuid.Parse(k)
				name, _ := v.(string)
				if err != nil || name == "" {
					continue
				}
				n.byID[id], n.byName[name] = name, id
			}
			return
		}
	}
	n.s.m.Log.Info("Generating player names map... This may take a while.")
	for _, id := range n.s.GlobalUUIDs() {
		n.set(id, n.s.Global(id, "").LastKnownName)
	}
	n.save()
	n.s.globals = map[plugin.UUID]*GlobalProfile{}
	n.s.m.Log.Info("Generated player names map.")
}

// set records a name, reporting whether it changed.
func (n *playerNames) set(id plugin.UUID, name string) bool {
	if name == "" || n.byName[name] == id && n.byID[id] == name {
		return false
	}
	if old, ok := n.byID[id]; ok {
		delete(n.byName, old)
	}
	n.byID[id], n.byName[name] = name, id
	return true
}

func (n *playerNames) save() {
	out := map[string]string{}
	for id, name := range n.byID {
		out[id.String()] = name
	}
	n.s.write(n.path(), out)
}

// Named answers the UUID of a player by last known name.
func (n *playerNames) Named(name string) (plugin.UUID, bool) {
	id, ok := n.byName[name]
	return id, ok
}

// All answers every player known, by name.
func (n *playerNames) All() []PlayerRef {
	out := make([]PlayerRef, 0, len(n.byName))
	for name, id := range n.byName {
		out = append(out, Ref{id, name})
	}
	slices.SortFunc(out, func(a, b PlayerRef) int { return strings.Compare(a.Name(), b.Name()) })
	return out
}

// NameOf answers a player's last known name.
func (n *playerNames) NameOf(id plugin.UUID) (string, bool) {
	name, ok := n.byID[id]
	return name, ok
}
