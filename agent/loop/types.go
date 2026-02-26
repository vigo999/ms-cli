package loop

// Event is emitted by runtime loop.
type Event struct {
	Type    string
	Payload any
}
