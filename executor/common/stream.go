package common

// StreamChunk is a chunk of stdout/stderr.
type StreamChunk struct {
	Source string
	Data   string
}
