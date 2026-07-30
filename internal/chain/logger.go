package chain

// Logger is the minimal logging interface shared by chain clients: a Debug level
// for opt-in diagnostics and an Error level for always-captured failures. Both
// use printf-style formatting.
type Logger interface {
	Debug(format string, args ...any)
	Error(format string, args ...any)
}
