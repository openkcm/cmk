package constants

// LogLevel represents available logging levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// String implements the Stringer interface
func (l LogLevel) String() string {
	return string(l)
}
