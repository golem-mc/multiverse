package core

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// WorldRef names a world either by its name or by its namespaced key, and
// answers both. MV: WorldKeyOrName
type WorldRef struct {
	// Name is the world's name: the level name for minecraft:overworld,
	// <level>_nether and <level>_the_end for the vanilla nether and end,
	// the key's path for another minecraft: key, and namespace_path for any
	// other key.
	Name string
	// Key is the dimension key the world has on the server.
	Key plugin.Key
	// IsKey reports a reference written as a key ("minecraft:creative"),
	// which worlds.yml then stores under the key.
	IsKey bool
}

// Serialise answers how worlds.yml names the world: by key or by name.
func (r WorldRef) Serialise() string {
	if r.IsKey {
		return r.Key.String()
	}
	return r.Name
}

func (r WorldRef) String() string { return r.Serialise() }

var (
	validWorldName = regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`)
	validKeyPart   = regexp.MustCompile(`^[a-z0-9_\-./]+$`)
)

// ParseWorldRef reads a world name or "namespace:key". MV:
// WorldKeyOrName.parse
func (c *Core) ParseWorldRef(s string) (WorldRef, error) {
	if s == "" {
		return WorldRef{}, MsgError(M("mv-core.worldkeyparse.empty"))
	}
	if strings.Contains(s, ":") {
		key, err := plugin.ParseKey(s)
		ns, path, _ := strings.Cut(s, ":")
		if err != nil || !validKeyPart.MatchString(ns) || !validKeyPart.MatchString(path) || strings.Count(s, ":") > 1 {
			return WorldRef{}, MsgError(M("mv-core.worldkeyparse.invalidnamespacedkey", "{namespace}", s))
		}
		return c.RefOfKey(key), nil
	}
	if !validWorldName.MatchString(s) {
		return WorldRef{}, MsgError(M("mv-core.worldkeyparse.invalidworldname", "{world}", s))
	}
	level := c.LevelName()
	path := strings.ToLower(s)
	switch {
	case strings.EqualFold(s, level):
		path = "overworld"
	case strings.EqualFold(s, defaultNetherFormat(level)):
		path = "the_nether"
	case strings.EqualFold(s, defaultEndFormat(level)):
		path = "the_end"
	}
	return WorldRef{Name: s, Key: plugin.Key{Namespace: "minecraft", Path: path}}, nil
}

// RefOfKey answers the reference to the world with key. MV:
// WorldKeyOrName.parseKey
func (c *Core) RefOfKey(key plugin.Key) WorldRef {
	r := WorldRef{Key: key, IsKey: true}
	if key.Namespace != "minecraft" {
		r.Name = key.Namespace + "_" + key.Path
		return r
	}
	level := c.LevelName()
	switch key.Path {
	case "overworld":
		r.Name = level
	case "the_nether":
		r.Name = defaultNetherFormat(level)
	case "the_end":
		r.Name = defaultEndFormat(level)
	default:
		r.Name = key.Path
	}
	return r
}

// defaultNetherFormat and defaultEndFormat are Bukkit's names for the
// vanilla nether and end. MV: DimensionFinder.DEFAULT_NETHER_FORMAT
func defaultNetherFormat(level string) string { return level + "_nether" }
func defaultEndFormat(level string) string    { return level + "_the_end" }

func (c *Config) netherName(overworld string) string {
	return strings.ReplaceAll(c.String("world.world-name-format.nether"), "%overworld%", overworld)
}

func (c *Config) endName(overworld string) string {
	return strings.ReplaceAll(c.String("world.world-name-format.end"), "%overworld%", overworld)
}

// LevelName answers server.properties' level-name, "world" when it cannot be
// read. MV: ServerProperties.getLevelName
func (c *Core) LevelName() string {
	if c.levelName != "" {
		return c.levelName
	}
	c.levelName = "world"
	path := c.Config.String("misc.server-properties-path")
	if path == "" {
		path = "server.properties"
	}
	f, err := os.Open(path)
	if err != nil {
		return c.levelName
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if v, ok := strings.CutPrefix(line, "level-name="); ok && v != "" {
			c.levelName = unescapeProperty(v)
		}
	}
	return c.levelName
}

// WorldFolder answers the directory a world's files live in, whether or not
// it is loaded; the overworld's is the level directory. MV: WorldFolderResolver
func (c *Core) WorldFolder(key plugin.Key) string {
	return c.Server.WorldFolder(key)
}

// PotentialWorlds answers the names of world folders Multiverse does not
// know, for /mv import. MV: WorldManager.getPotentialWorlds
func (c *Core) PotentialWorlds() []string {
	root := filepath.Join(c.WorldFolder(plugin.MinecraftKey("overworld")), "dimensions")
	namespaces, _ := os.ReadDir(root)
	var out []string
	for _, ns := range namespaces {
		paths, _ := os.ReadDir(filepath.Join(root, ns.Name()))
		for _, p := range paths {
			key := plugin.Key{Namespace: ns.Name(), Path: p.Name()}
			if !p.IsDir() || !c.HasWorldFolder(key) {
				continue
			}
			ref := c.RefOfKey(key)
			if _, known := c.Worlds.byRef(ref); !known {
				out = append(out, ref.Name)
			}
		}
	}
	return out
}

// storageFolder answers where a world's chunks and saved data are: its
// folder, but for the overworld, whose folder is the level directory,
// <level>/dimensions/minecraft/overworld.
func (c *Core) storageFolder(key plugin.Key) string {
	if key == plugin.MinecraftKey("overworld") {
		return filepath.Join(c.WorldFolder(key), "dimensions", "minecraft", "overworld")
	}
	return c.WorldFolder(key)
}

// HasWorldFolder reports whether a world's folder holds a world: a region or
// data folder. MV: WorldNameChecker.checkFolder
func (c *Core) HasWorldFolder(key plugin.Key) bool {
	dir := c.storageFolder(key)
	for _, sub := range []string{"region", "data", "entities"} {
		if st, err := os.Stat(filepath.Join(dir, sub)); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}
