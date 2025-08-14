package ra

import "os"

// ExitFunc is the interface for exiting the program
type ExitFunc func(int)

// StderrWriter is the interface for writing to stderr
type StderrWriter interface {
	Write([]byte) (int, error)
}

var osExit ExitFunc = os.Exit
var stderrWriter StderrWriter = os.Stderr

// SetStderrWriter allows overriding the stderr writer for testing or custom output
func SetStderrWriter(writer StderrWriter) {
	stderrWriter = writer
}

// SetExitFunc allows overriding the exit function for testing
func SetExitFunc(exitFunc ExitFunc) {
	osExit = exitFunc
}
