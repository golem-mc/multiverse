package mvtest

import (
	"sync"

	"github.com/golem-mc/golem/plugin"
)

// perms is a permission provider for tests: nodes set per player name,
// everything else left to the defaults.
type perms struct {
	mu    sync.Mutex
	nodes map[string]map[string]bool
}

func (p *perms) Has(sender plugin.CommandSender, node string) (has, decided bool) {
	pl, ok := sender.(plugin.Player)
	if !ok {
		return false, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	has, decided = p.nodes[pl.Name()][node]
	return has, decided
}

// Permit grants (or with value false, denies) nodes to the player named,
// as a permissions plugin would.
func (env *Env) Permit(name string, value bool, nodes ...string) {
	env.perms.mu.Lock()
	defer env.perms.mu.Unlock()
	m := env.perms.nodes[name]
	if m == nil {
		m = map[string]bool{}
		env.perms.nodes[name] = m
	}
	for _, n := range nodes {
		m[n] = value
	}
}
