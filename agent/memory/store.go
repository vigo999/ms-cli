package memory

import (
	"time"
)

// Store is memory persistence abstraction.
type Store interface {
	// Save saves a memory item (insert or update).
	Save(item *MemoryItem) error

	// Get retrieves a memory item by ID.
	Get(id string) (*MemoryItem, error)

	// Query retrieves memory items matching the query.
	Query(query Query) ([]*MemoryItem, error)

	// Delete removes a memory item by ID.
	Delete(id string) error

	// DeleteBefore removes all memory items created before the given time.
	DeleteBefore(t time.Time) error

	// DeleteExpired removes all expired memory items.
	DeleteExpired() error

	// Close closes the store.
	Close() error
}

// ExtendedStore 扩展存储接口
type ExtendedStore interface {
	Store

	// List lists memory items with pagination.
	List(limit, offset int) ([]*MemoryItem, error)

	// Count returns the total number of memory items.
	Count() (int64, error)

	// CountByType returns the count of memory items by type.
	CountByType() (map[MemoryType]int64, error)

	// DeleteByType removes all memory items of a specific type.
	DeleteByType(memType MemoryType) error

	// GetStats returns statistics about the stored memories.
	GetStats() (MemoryStats, error)
}
