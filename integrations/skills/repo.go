package skills

// RepoSync manages skills repository sync.
type RepoSync interface {
	Sync() error
}
