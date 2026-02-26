package memory

// Store is memory persistence abstraction.
type Store interface {
	Put(key, value string) error
	Get(key string) (string, error)
}
