package core

import (
	"fmt"
	"io"
	"io/fs"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// CreateOptions are /mv create's: a new world. MV: CreateWorldOptions
type CreateOptions struct {
	Name        string
	Environment string
	// Seed is the seed as typed: a number, else hashed as Java's
	// String.hashCode; empty for a random one.
	Seed              string
	Generator         string
	GeneratorSettings string
	// WorldType is normal, flat, large_biomes or amplified.
	WorldType  string
	Biome      string
	NoAdjust   bool
	NoStruct   bool
	BonusChest bool
	// SpawnAt forces the spawn position.
	SpawnAt *SpawnLocation
	// Properties are world properties to set, by name.
	Properties map[string]string
	// SkipFolderCheck creates the world even when a folder exists for it.
	SkipFolderCheck bool
}

// WorldTypes are Bukkit's world types, lowercase.
var WorldTypes = []string{"normal", "flat", "large_biomes", "amplified"}

// ParseSeed reads a seed as WorldCreator.seed(String) does.
func ParseSeed(s string) int64 {
	if s == "" {
		for {
			if n := rand.Int64() - rand.Int64(); n != 0 {
				return n
			}
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	var h int32
	for _, r := range utf16Units(s) {
		h = 31*h + int32(r)
	}
	return int64(h)
}

func utf16Units(s string) []uint16 {
	var out []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xd800+(r>>10)), uint16(0xdc00+(r&0x3ff)))
			continue
		}
		out = append(out, uint16(r))
	}
	return out
}

// checkNew refuses a world reference that names a known world or, unless
// skipFolder, an existing folder; kind is the message group
// (createworld, importworld, cloneworld).
func (ws *Worlds) checkNew(name, kind string, folderMustNotExist bool) (WorldRef, error) {
	ref, err := ws.core.ParseWorldRef(name)
	if err != nil {
		return ref, err
	}
	if w, ok := ws.byRef(ref); ok {
		if w.Loaded() {
			return ref, MsgError(M("mv-core."+kind+".worldexistloaded", "{world}", ref.Name))
		}
		return ref, MsgError(M("mv-core."+kind+".worldexistunloaded", "{world}", ref.Name))
	}
	if _, ok := ws.core.Server.World(ref.Key); ok && kind != "importworld" {
		return ref, MsgError(M("mv-core."+kind+".worldexistloaded", "{world}", ref.Name))
	}
	if folderMustNotExist && ws.core.HasWorldFolder(ref.Key) {
		return ref, MsgError(M("mv-core."+kind+".worldexistfolder", "{world}", ref.Name))
	}
	return ref, nil
}

// unsupported refuses the world-creation options golem cannot honour yet.
func unsupported(o *CreateOptions) error {
	var what string
	switch {
	case o.WorldType != "" && o.WorldType != "normal":
		what = "world type " + strings.ToUpper(o.WorldType)
	case o.GeneratorSettings != "":
		what = "generator settings"
	case o.Biome != "":
		what = "a single-biome world"
	case o.NoStruct:
		what = "turning structures off"
	case o.BonusChest:
		what = "a bonus chest"
	default:
		return nil
	}
	return fmt.Errorf("%s is not supported by golem yet (golem-mc/golem#66)", what)
}

// Create makes and loads a new world. MV: WorldManager.createWorld
func (ws *Worlds) Create(o CreateOptions) (*World, error) {
	ref, err := ws.checkNew(o.Name, "createworld", !o.SkipFolderCheck)
	if err != nil {
		return nil, err
	}
	if err := unsupported(&o); err != nil {
		return nil, MsgError(M("mv-core.worldcreator.bukkitcreationfailed", "{world}", ref.Name, "{error}", err.Error()))
	}
	opts, err := ws.options(ref.Key, o.Environment, o.Generator)
	if err != nil {
		return nil, err
	}
	opts.Seed = ParseSeed(o.Seed)
	h, err := ws.core.Server.CreateWorld(opts)
	if err != nil {
		return nil, MsgError(M("mv-core.worldcreator.bukkitcreationfailed", "{world}", ref.Name, "{error}", err.Error()))
	}
	w := ws.newLoaded(ref, h, o.Generator, o.Biome, o.GeneratorSettings, !o.NoAdjust)
	if o.SpawnAt != nil {
		ws.SetSpawn(w, o.SpawnAt.Location(h))
	}
	for name, value := range o.Properties {
		if err := w.Set(name, value); err != nil {
			ws.core.Log.Warn(fmt.Sprintf("Failed to set property '%s' to '%s' for world %s: %s", name, value, w.Name(), err))
		}
	}
	ws.core.fire(&MVWorldCreatedEvent{World: w})
	return w, ws.Save()
}

// newLoaded adds a world that is loaded as h, with the options it was made
// with. MV: WorldManager.newLoadedMultiverseWorld
func (ws *Worlds) newLoaded(ref WorldRef, h plugin.World, generator, biome, settings string, adjust bool) *World {
	w := ws.add(ref)
	w.Props.setRaw(PropAdjustSpawn, adjust)
	w.Props.setRaw(PropGenerator, generator)
	w.Props.setRaw(PropBiome, biome)
	w.Props.setRaw(PropGeneratorSettings, settings)
	w.Props.setRaw(PropDifficulty, Difficulties[h.Difficulty()])
	w.Props.setRaw(PropScale, defaultScale(EnvironmentOfDimension(h.Dimension())))
	ws.attach(w, h)
	_ = ws.Save()
	return w
}

// SetSpawn moves a world's spawn: in worlds.yml and, for a world of its
// own, on the server. The vanilla nether and end share the overworld's
// spawn on golem, so theirs is kept only in worlds.yml. MV:
// MultiverseWorld.setSpawnLocation
func (ws *Worlds) SetSpawn(w *World, loc plugin.Location) {
	w.Props.setRaw(PropSpawnLocation, SpawnLocationOf(loc))
	if h, ok := w.Handle(); ok && !sharesSpawn(h) {
		loc.World = h
		h.SetSpawnPoint(loc)
	}
}

// sharesSpawn reports a vanilla nether or end, whose spawn is the shared
// world spawn on golem.
func sharesSpawn(h plugin.World) bool {
	k := h.Key()
	return k == plugin.MinecraftKey("the_nether") || k == plugin.MinecraftKey("the_end")
}

// ImportOptions are /mv import's: an existing world. MV: ImportWorldOptions
type ImportOptions struct {
	Name, Environment, Generator, Biome, GeneratorSettings string
	NoAdjust, SkipFolderCheck                              bool
}

// Import adds an existing world and loads it. MV: WorldManager.importWorld
func (ws *Worlds) Import(o ImportOptions) (*World, error) {
	ref, err := ws.checkNew(o.Name, "importworld", false)
	if err != nil {
		return nil, err
	}
	if h, ok := ws.core.Server.World(ref.Key); ok {
		if env := EnvironmentOfDimension(h.Dimension()); env != strings.ToLower(o.Environment) {
			return nil, MsgError(M("mv-core.importworld.bukkitenvironmentmismatch", "{world}", ref.Name,
				"{bukkitEnvironment}", strings.ToUpper(env), "{mvEnvironment}", strings.ToUpper(o.Environment)))
		}
		w := ws.newLoaded(ref, h, o.Generator, o.Biome, o.GeneratorSettings, !o.NoAdjust)
		ws.core.fire(&MVWorldImportedEvent{World: w})
		return w, nil
	}
	if !o.SkipFolderCheck && !ws.core.HasWorldFolder(ref.Key) {
		return nil, MsgError(M("mv-core.importworld.worldfolderinvalid", "{world}", ref.Name))
	}
	if err := unsupported(&CreateOptions{Biome: o.Biome, GeneratorSettings: o.GeneratorSettings}); err != nil {
		return nil, MsgError(M("mv-core.worldcreator.bukkitcreationfailed", "{world}", ref.Name, "{error}", err.Error()))
	}
	opts, err := ws.options(ref.Key, o.Environment, o.Generator)
	if err != nil {
		return nil, err
	}
	h, err := ws.core.Server.CreateWorld(opts)
	if err != nil {
		return nil, MsgError(M("mv-core.worldcreator.bukkitcreationfailed", "{world}", ref.Name, "{error}", err.Error()))
	}
	w := ws.newLoaded(ref, h, o.Generator, o.Biome, o.GeneratorSettings, !o.NoAdjust)
	ws.core.fire(&MVWorldImportedEvent{World: w})
	return w, nil
}

// LoadChecked loads a known world, refusing first when its folder is gone
// unless skipFolderCheck. MV: WorldManager.loadWorld with LoadWorldOptions
func (ws *Worlds) LoadChecked(w *World, skipFolderCheck bool) (*World, error) {
	if !w.Loaded() && !skipFolderCheck && !isVanilla(w.Key()) {
		if _, loaded := ws.core.Server.World(w.Key()); !loaded && !ws.core.HasWorldFolder(w.Key()) {
			return nil, MsgError(M("mv-core.deleteworld.loadfailed", "{world}", w.Name()))
		}
	}
	return ws.Load(w)
}

func isVanilla(k plugin.Key) bool {
	return k.Namespace == "minecraft" && (k.Path == "overworld" || k.Path == "the_nether" || k.Path == "the_end")
}

// RemovePlayers teleports every player in w to dest. MV:
// PlayerWorldTeleporter.transferAllFromWorldToDestination
func (ws *Worlds) RemovePlayers(w *World, dest Destination) error {
	failed := false
	for _, p := range w.Players() {
		loc, ok := dest.Location(p)
		if !ok || !p.Teleport(loc, plugin.TeleportPlugin) {
			failed = true
		}
	}
	if failed {
		return MsgError(M("mv-core.generic.teleportplayers.failed"))
	}
	return nil
}

// UnloadOptions are /mv unload's and /mv remove's. MV: UnloadWorldOptions
type UnloadOptions struct {
	// KeepServerWorld leaves the world loaded on the server, only
	// forgetting it in Multiverse (--no-unload-bukkit-world).
	KeepServerWorld bool
	NoSave          bool
}

// UnloadWith unloads w as the options say. MV: WorldManager.unloadWorld
func (ws *Worlds) UnloadWith(w *World, o UnloadOptions) error {
	if !w.Loaded() {
		return MsgError(M("mv-core.unloadworld.worldunloaded", "{world}", w.Name()))
	}
	if o.KeepServerWorld {
		w.handle = nil
		ws.core.fire(&MVWorldUnloadedEvent{World: w})
		return nil
	}
	return ws.Unload(w, !o.NoSave)
}

// Remove unloads w (when loaded) and forgets it, leaving its files. MV:
// WorldManager.removeWorld
func (ws *Worlds) Remove(w *World, o UnloadOptions) error {
	if w.Loaded() {
		if err := ws.UnloadWith(w, o); err != nil {
			return err
		}
	}
	ws.remove(w)
	ws.core.fire(&MVWorldRemovedEvent{Name: w.Name()})
	return ws.Save()
}

// Delete unloads and forgets w and deletes its folder, keeping the files
// keep names (paths relative to the folder). An unloaded world is loaded
// first, to be sure it is a world. MV: WorldManager.deleteWorld
func (ws *Worlds) Delete(w *World, keep []string) error {
	if !w.Loaded() {
		if _, err := ws.Load(w); err != nil {
			return MsgError(M("mv-core.deleteworld.loadfailed", "{world}", w.Name()))
		}
	}
	if isVanilla(w.Key()) {
		return MsgError(M("mv-core.exception.multiverseworld.unloaddefaultworld", "{world}", w.Name()))
	}
	folder := ws.core.WorldFolder(w.Key())
	if _, err := os.Stat(folder); err != nil {
		return MsgError(M("mv-core.deleteworld.worldfoldernotfound", "{world}", w.Name()))
	}
	e := &MVWorldDeleteEvent{World: w}
	ws.core.fire(e)
	if e.Cancelled() {
		return MsgError(M("mv-core.generic.failure"))
	}
	if err := ws.Remove(w, UnloadOptions{NoSave: true}); err != nil {
		return err
	}
	if err := deleteFolder(folder, keep); err != nil {
		return MsgError(M("mv-core.deleteworld.failedtodeletefolder", "{world}", w.Name(), "{error}", err.Error()))
	}
	return nil
}

// deleteFolder removes dir except the paths in keep. MV: FileUtils.deleteFolder
func deleteFolder(dir string, keep []string) error {
	if len(keep) == 0 {
		return os.RemoveAll(dir)
	}
	kept := map[string]bool{}
	for _, k := range keep {
		kept[filepath.Clean(k)] = true
	}
	var dirs []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if kept[rel] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		return os.Remove(path)
	})
	for _, dir := range slices.Backward(dirs) {
		_ = os.Remove(dir) // fails, as it should, while a kept file is inside
	}
	return err
}

// cloneIgnore are files a clone does not copy: the source's lock and
// identity, and golem's record of the source's seed and spawn.
// MV: WorldManager.CLONE_IGNORE_FILES
var cloneIgnore = []string{"uid.dat", "session.lock", filepath.Join("data", "plugin_world.dat")}

// CloneOptions are /mv clone's. MV: CloneWorldOptions
type CloneOptions struct {
	From                                             *World
	NewName                                          string
	ResetConfig, ResetGameRules, ResetBorder, NoSave bool
}

// Clone copies a world's folder to a new world and imports it, with the
// source's properties unless ResetConfig. MV: WorldManager.cloneWorld
func (ws *Worlds) Clone(o CloneOptions) (*World, error) {
	ref, err := ws.checkNew(o.NewName, "cloneworld", true)
	if err != nil {
		return nil, err
	}
	from := ws.core.storageFolder(o.From.Key())
	if !ws.core.HasWorldFolder(o.From.Key()) {
		return nil, MsgError(M("mv-core.cloneworld.fromworldfolderinvalid", "{world}", o.From.Name()))
	}
	if h, ok := o.From.Handle(); ok && !o.NoSave {
		h.Save(true)
	}
	if err := copyFolder(from, ws.core.WorldFolder(ref.Key), cloneIgnore); err != nil {
		return nil, MsgError(M("mv-core.cloneworld.copyfailed", "{world}", o.From.Name(), "{error}", err.Error()))
	}
	seed, _ := o.From.Seed()
	opts, err := ws.options(ref.Key, o.From.Environment(), o.From.Generator())
	if err != nil {
		return nil, err
	}
	opts.Seed = seed
	h, err := ws.core.Server.CreateWorld(opts)
	if err != nil {
		return nil, MsgError(M("mv-core.worldcreator.bukkitcreationfailed", "{world}", ref.Name, "{error}", err.Error()))
	}
	w := ws.newLoaded(ref, h, o.From.Generator(), o.From.Props.String(PropBiome), o.From.Props.String(PropGeneratorSettings), o.From.Props.Bool(PropAdjustSpawn))
	if !o.ResetConfig {
		ws.copyProps(o.From, w)
		if s := o.From.SpawnLocation(); s != nil {
			ws.SetSpawn(w, s.Location(h))
		}
	}
	ws.transferBorder(o.From, w, !o.ResetBorder)
	ws.core.fire(&MVWorldClonedEvent{World: w, From: o.From})
	return w, ws.Save()
}

// copyProps copies the visible properties (what /mv modify sets) of from to
// to. MV: WorldConfigStore
func (ws *Worlds) copyProps(from, to *World) {
	for _, n := range from.Props.Nodes().All() {
		if n.Kind == KindHeader || n.Hidden {
			continue
		}
		if tn, ok := to.Props.Nodes().Find(n.NameOrPath()); ok {
			_ = to.Props.Set(tn, cloneValue(from.Props.Get(n)))
		}
	}
	to.Props.setRaw(PropSpawning, cloneSpawning(from.Spawning()))
	to.Props.setRaw(PropMeta, cloneMeta(from.Meta()))
}

func cloneValue(v any) any {
	if l, ok := v.([]string); ok {
		return append([]string(nil), l...)
	}
	return v
}

func cloneMeta(m Meta) Meta {
	out := Meta{}
	maps.Copy(out, m)
	return out
}

func cloneSpawning(s *Spawning) *Spawning {
	out := NewSpawning()
	for name, v := range s.Categories {
		c := out.Category(name)
		for _, n := range v.Nodes().All() {
			c.setRaw(n.Path, cloneValue(v.Get(n)))
		}
	}
	return out
}

// transferBorder gives to the border of from when keep is set, else
// vanilla's default border. MV: WorldBorderStore, WorldBorder.reset
func (ws *Worlds) transferBorder(from, to *World, keep bool) {
	th, ok := to.Handle()
	if !ok {
		return
	}
	b := th.Border()
	if !keep {
		b.SetCenter(0, 0)
		b.SetSize(5.9999968e7, 0)
		b.SetDamagePerBlock(0.2)
		b.SetWarningDistance(5)
		return
	}
	fh, ok := from.Handle()
	if !ok {
		return
	}
	fb := fh.Border()
	x, z := fb.Center()
	b.SetCenter(x, z)
	b.SetSize(fb.Size(), 0)
	b.SetDamagePerBlock(fb.DamagePerBlock())
	b.SetWarningDistance(fb.WarningDistance())
}

// copyFolder copies src to dst, skipping the relative paths in ignore.
func copyFolder(src, dst string, ignore []string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if slices.Contains(ignore, rel) {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// RegenOptions are /mv regen's. MV: RegenWorldOptions
type RegenOptions struct {
	World *World
	// Seed is the new seed as typed; RandomSeed picks one; neither keeps
	// the world's.
	Seed                                     string
	RandomSeed                               bool
	ResetConfig, ResetGameRules, ResetBorder bool
	KeepFiles                                []string
}

// Regen deletes a world and creates it again, with its properties and
// border unless reset. MV: WorldManager.regenWorld
func (ws *Worlds) Regen(o RegenOptions) (*World, error) {
	w := o.World
	oldSeed, _ := w.Seed()
	seed := strconv.FormatInt(oldSeed, 10)
	switch {
	case o.Seed != "":
		seed = o.Seed
	case o.RandomSeed:
		seed = ""
	}
	newSeed := ParseSeed(seed)
	keepSpawn := !o.ResetConfig && newSeed == oldSeed
	spawn := w.SpawnLocation()
	saved := ws.core.newWorld(w.Ref())
	ws.copyProps(w, saved)
	var border struct {
		x, z, size, damage float64
		warn               int
	}
	if h, ok := w.Handle(); ok {
		b := h.Border()
		border.x, border.z = b.Center()
		border.size, border.damage, border.warn = b.Size(), b.DamagePerBlock(), b.WarningDistance()
	}
	create := CreateOptions{Name: w.Name(), Environment: w.Environment(), Seed: strconv.FormatInt(newSeed, 10),
		Generator: w.Generator(), NoAdjust: keepSpawn || !w.Props.Bool(PropAdjustSpawn),
		SkipFolderCheck: len(o.KeepFiles) > 0}
	if err := ws.Delete(w, o.KeepFiles); err != nil {
		return nil, err
	}
	nw, err := ws.Create(create)
	if err != nil {
		return nil, err
	}
	if !o.ResetConfig {
		ws.copyProps(saved, nw)
	}
	if h, ok := nw.Handle(); ok && !o.ResetBorder && border.size > 0 {
		b := h.Border()
		b.SetCenter(border.x, border.z)
		b.SetSize(border.size, 0)
		b.SetDamagePerBlock(border.damage)
		b.SetWarningDistance(border.warn)
	}
	if keepSpawn && spawn != nil {
		if h, ok := nw.Handle(); ok {
			ws.SetSpawn(nw, spawn.Location(h))
		}
	}
	ws.core.fire(&MVWorldRegeneratedEvent{World: nw})
	return nw, ws.Save()
}
