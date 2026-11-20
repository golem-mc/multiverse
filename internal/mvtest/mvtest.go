// Package mvtest starts a golem test server (plugintest) with Multiverse
// enabled, joins fake players and reads what they are sent, for the
// black-box tests of the commands and modules.
package mvtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/plugintest"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
)

// VoidGenerator is a generator that makes empty chunks and spawns at
// (0, 64, 0): the cheapest world a test can create. Tests use it as
// "-g Void".
type VoidGenerator struct{}

func (VoidGenerator) Generate(plugin.ChunkWriter, plugin.Random)                   {}
func (VoidGenerator) Populate(plugin.RegionWriter, plugin.ChunkPos, plugin.Random) {}
func (VoidGenerator) SpawnPoint(w plugin.World) plugin.Location {
	return plugin.Location{World: w, Pos: plugin.Vec3{X: 0, Y: 64, Z: 0}}
}

// FloorGenerator is a stone floor at y 0..3 that spawns players on it.
type FloorGenerator struct{}

var stone = plugin.DefaultState(registry.BlockStone)

func (FloorGenerator) Generate(c plugin.ChunkWriter, _ plugin.Random) {
	for x := range 16 {
		for z := range 16 {
			for y := range 4 {
				c.Set(x, y, z, stone)
			}
		}
	}
}
func (FloorGenerator) Populate(plugin.RegionWriter, plugin.ChunkPos, plugin.Random) {}
func (FloorGenerator) SpawnPoint(w plugin.World) plugin.Location {
	return plugin.Location{World: w, Pos: plugin.Vec3{X: 8.5, Y: 4, Z: 8.5}}
}

func init() {
	core.RegisterGenerator("Void", func(plugin.Key, string) plugin.Generator { return VoidGenerator{} })
	core.RegisterGenerator("Floor", func(plugin.Key, string) plugin.Generator { return FloorGenerator{} })
}

// Env is a running test server with Multiverse.
type Env struct {
	T   *testing.T
	Srv *plugintest.Server
	MV  *core.Core
	// Dir is the server's working directory; DataDir is Multiverse's.
	Dir, DataDir string
}

// Start starts a flat test server in a temporary directory with the plugins
// the test binary registered. files are written under plugins/multiverse-core
// first (name to contents), as an admin's existing config would be.
func Start(t *testing.T, files map[string]string, opts ...plugintest.Option) *Env {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	data := filepath.Join(dir, "plugins", "multiverse-core")
	for name, content := range files {
		path := filepath.Join(data, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	srv := plugintest.Start(t, opts...)
	c, ok := core.Get()
	if !ok {
		t.Fatal("multiverse-core did not enable")
	}
	if _, ok := c.Server.(interface{ WorldFolder(plugin.Key) string }); !ok {
		pointWorldFolders(c, filepath.Dir(dir))
	}
	return &Env{T: t, Srv: srv, MV: c, Dir: dir, DataDir: data}
}

// pointWorldFolders makes c find worlds in the test server's level
// directory, the sibling temp dir holding level.dat, until golem answers a
// world's folder itself (golem-mc/golem#63).
func pointWorldFolders(c *core.Core, parent string) {
	entries, _ := os.ReadDir(parent)
	for _, e := range entries {
		level := filepath.Join(parent, e.Name())
		if _, err := os.Stat(filepath.Join(level, "level.dat")); err != nil {
			continue
		}
		c.SetWorldFolderFunc(func(k plugin.Key) string {
			if k == plugin.MinecraftKey("overworld") {
				return level
			}
			return filepath.Join(level, "dimensions", k.Namespace, k.Path)
		})
		return
	}
}

// Sync runs fn on the tick goroutine and returns once it has run.
func (env *Env) Sync(fn func()) {
	env.T.Helper()
	done := make(chan struct{})
	env.MV.Server.Scheduler().RunTask(func() { fn(); close(done) })
	for range 200 {
		select {
		case <-done:
			return
		default:
			env.Srv.RunTicks(1)
		}
	}
	env.T.Fatal("Sync: the task did not run within 200 ticks")
}

// Console runs a console command line (no slash) and answers its output.
func (env *Env) Console(line string) []string {
	env.T.Helper()
	return env.Srv.Console(line)
}

// Join joins a player; op makes them an operator of level 4.
func (env *Env) Join(name string, op bool) *plugintest.Player {
	env.T.Helper()
	p := env.Srv.Join(name)
	if op {
		p.Op()
	}
	p.Messages(3)
	return p
}

// Run runs a command (no slash) as p and answers the messages they get in 5
// ticks.
func Run(p *plugintest.Player, line string) []string {
	p.Run("/" + line)
	return p.Messages(5)
}

// ReadFile answers a file under Multiverse's data directory.
func (env *Env) ReadFile(name string) string {
	env.T.Helper()
	data, err := os.ReadFile(filepath.Join(env.DataDir, name))
	if err != nil {
		env.T.Fatal(err)
	}
	return string(data)
}

// Has reports whether any line contains sub.
func Has(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

// Expect fails unless a line contains sub.
func (env *Env) Expect(lines []string, sub string) {
	env.T.Helper()
	if !Has(lines, sub) {
		env.T.Fatalf("no line contains %q; got %q", sub, lines)
	}
}

// ExpectNot fails if a line contains sub.
func (env *Env) ExpectNot(lines []string, sub string) {
	env.T.Helper()
	if Has(lines, sub) {
		env.T.Fatalf("a line contains %q; got %q", sub, lines)
	}
}
