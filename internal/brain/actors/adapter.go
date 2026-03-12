package actors

import "github.com/dohr-michael/ozzie/internal/agent"

// PoolAdapter wraps ActorPool to implement agent.CapacityPool.
type PoolAdapter struct {
	pool *ActorPool
}

// NewPoolAdapter creates a PoolAdapter for the given ActorPool.
func NewPoolAdapter(pool *ActorPool) *PoolAdapter {
	return &PoolAdapter{pool: pool}
}

// AcquireInteractive acquires a slot and returns it as agent.CapacitySlot.
func (a *PoolAdapter) AcquireInteractive(providerName string) (agent.CapacitySlot, error) {
	return a.pool.AcquireInteractive(providerName)
}

// Release frees a slot.
func (a *PoolAdapter) Release(slot agent.CapacitySlot) {
	if actor, ok := slot.(*Actor); ok {
		a.pool.Release(actor)
	}
}

var _ agent.CapacityPool = (*PoolAdapter)(nil)
