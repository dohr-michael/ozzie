package organisms

// Mode represents the current interaction state.
type Mode int

const (
	ModeNormal    Mode = iota
	ModeStreaming       // receiving assistant stream
	ModePrompting       // waiting for user prompt response
)
