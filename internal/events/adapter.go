package events

import "io"

// TranscriptAdapter parses agent-specific transcript formats into Events.
type TranscriptAdapter interface {
	Parse(reader io.Reader) ([]Event, error)
	Format() string
}

var adapters = map[string]TranscriptAdapter{}

// RegisterAdapter registers a TranscriptAdapter by its format name.
func RegisterAdapter(a TranscriptAdapter) {
	adapters[a.Format()] = a
}

// GetAdapter returns the adapter registered for the given format, if any.
func GetAdapter(format string) (TranscriptAdapter, bool) {
	a, ok := adapters[format]
	return a, ok
}

// AvailableFormats returns the names of all registered adapter formats.
func AvailableFormats() []string {
	formats := make([]string, 0, len(adapters))
	for f := range adapters {
		formats = append(formats, f)
	}
	return formats
}
