package commands_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/vault"

	"github.com/golem-mc/multiverse/internal/mvtest"
)

// bank is a vault.Economy with balances in memory.
type bank struct {
	mu       sync.Mutex
	balances map[plugin.UUID]float64
}

func (*bank) Name() string                                       { return "Test" }
func (*bank) FractionalDigits() int                              { return 2 }
func (*bank) Format(a float64) string                            { return fmt.Sprintf("$%.2f", a) }
func (*bank) CurrencyNameSingular() string                       { return "dollar" }
func (*bank) CurrencyNamePlural() string                         { return "dollars" }
func (*bank) HasAccount(plugin.UUID) bool                        { return true }
func (*bank) CreateAccount(plugin.UUID, string) bool             { return true }
func (b *bank) Has(id plugin.UUID, a float64) bool               { return b.Balance(id) >= a }
func (b *bank) Deposit(id plugin.UUID, a float64) vault.Response { return b.Withdraw(id, -a) }

func (b *bank) Balance(id plugin.UUID) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.balances[id]
}

func (b *bank) Withdraw(id plugin.UUID, a float64) vault.Response {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.balances[id] -= a
	return vault.Response{Amount: a, Balance: b.balances[id]}
}

// An @vault-economy entry fee is checked and charged through vault.Economy.
func TestEconomyEntryFee(t *testing.T) {
	env := mvtest.Start(t, nil)
	env.Console("mv create Arena normal -g Floor")
	bob := env.Join("Bob", false)
	env.Permit("Bob", true, "multiverse.teleport.self.pl.Bob", "multiverse.teleport.self.w.Arena")
	arena, _ := env.MV.Worlds.Get("Arena")
	env.Sync(func() {
		_ = arena.Set("entryfee-enabled", "true")
		_ = arena.Set("entryfee-amount", "5")
	})
	env.Expect(mvtest.Run(bob, "mv tp Arena"), "You do not have the ability to pay entry fee.")

	b := &bank{balances: map[plugin.UUID]float64{}}
	vault.ProvideEconomy(env.MV.Server, b, vault.PriorityNormal)
	id := bob.Handle().UUID()
	b.Deposit(id, 3)
	env.Expect(mvtest.Run(bob, "mv tp Arena"), "You are required to pay $5.00.")
	b.Deposit(id, 7)
	out := mvtest.Run(bob, "mv tp Arena")
	env.Expect(out, "You have been charged $5.00.")
	env.Expect(out, "Teleported you to Arena.")
	if got := b.Balance(id); got != 5 {
		t.Errorf("balance %v, want 5", got)
	}
	env.Expect(env.Console("mv info Arena"), "Entry Fee: $5.00")
}
