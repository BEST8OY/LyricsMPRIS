package logutil

import (
	"fmt"
	"os"
)

var Verbose bool

func LogVerbose(format string, args ...interface{}) {
	if Verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
