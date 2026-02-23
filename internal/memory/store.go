package memory

// Store defines the interface for memory persistence.
type Store interface {
	Create(entry *MemoryEntry, content string) error
	Get(id string) (*MemoryEntry, string, error)
	Update(entry *MemoryEntry, content string) error
	Delete(id string) error
	List() ([]*MemoryEntry, error)
}
