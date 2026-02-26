package skills

// Invoker triggers skill workflows.
type Invoker interface {
	Invoke(name string) error
}
