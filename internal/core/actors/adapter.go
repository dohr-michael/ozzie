package actors

import "github.com/dohr-michael/ozzie/internal/core/brain"

// PoolAdapter wraps ActorPool to implement brain.CapacityPool.
type PoolAdapter struct {
	pool *ActorPool
}

// NewPoolAdapter creates a PoolAdapter for the given ActorPool.
func NewPoolAdapter(pool *ActorPool) *PoolAdapter {
	return &PoolAdapter{pool: pool}
}

// AcquireInteractive acquires a slot and returns it as brain.CapacitySlot.
func (a *PoolAdapter) AcquireInteractive(providerName string) (brain.CapacitySlot, error) {
	return a.pool.AcquireInteractive(providerName)
}

// Release frees a slot.
func (a *PoolAdapter) Release(slot brain.CapacitySlot) {
	if actor, ok := slot.(*Actor); ok {
		a.pool.Release(actor)
	}
}

var _ brain.CapacityPool = (*PoolAdapter)(nil)
