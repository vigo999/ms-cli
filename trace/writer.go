package trace

// Writer writes structured runtime events.
type Writer interface {
	Write(eventType string, payload any) error
}
