package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"time"
)

var vv = VV

// for tons of debug output
var VerboseVerbose bool = false

var NYC *time.Location

var NoTicksAvail = fmt.Errorf("no ticks available")
var ErrTimePointInFuture = fmt.Errorf("Tm pt in future")

func init() {
	var err error
	NYC, err = time.LoadLocation("America/New_York")
	panicOn(err)
}

func P(format string, a ...interface{}) {
	if Verbose {
		TSPrintf(format, a...)
	}
}

func PP(format string, a ...interface{}) {
	if VerboseVerbose {
		TSPrintf(format, a...)
	}
}

func VV(format string, a ...interface{}) {
	TSPrintf(format, a...)
}

// without the file/line, otherwise the same as PP
func PPP(format string, a ...interface{}) {
	if VerboseVerbose {
		Printf("\n%s ", ts())
		Printf(format+"\n", a...)
	}
}

func PB(w io.Writer, format string, a ...interface{}) {
	if Verbose {
		fmt.Fprintf(w, "\n"+format+"\n", a...)
	}
}

func AlwaysPrintf(format string, a ...interface{}) {
	TSPrintf(format, a...)
}

// time-stamped printf
func TSPrintf(format string, a ...interface{}) {
	Printf("\n%s %s ", FileLine(3), ts())
	Printf(format+"\n", a...)
}

// get timestamp for logging purposes
func ts() string {
	return time.Now().In(NYC).Format("2006-01-02 15:04:05.999 -0700 MST")
}

// so we can multi write easily, use our own printf
var OurStdout io.Writer = os.Stdout

// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.
func Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(OurStdout, format, a...)
}

func FileLine(depth int) string {
	_, fileName, fileLine, ok := runtime.Caller(depth)
	var s string
	if ok {
		s = fmt.Sprintf("%s:%d", path.Base(fileName), fileLine)
	} else {
		s = ""
	}
	return s
}

func p(format string, a ...interface{}) {
	if Verbose {
		TSPrintf(format, a...)
	}
}

var pp = PP

func pb(w io.Writer, format string, a ...interface{}) {
	if Verbose {
		fmt.Fprintf(w, "\n"+format+"\n", a...)
	}
}

// quieted for now, uncomment below to display
func VPrintf(format string, a ...interface{}) (n int, err error) {
	//return fmt.Fprintf(OurStdout, format, a...)
	return
}

func QPrintf(format string, a ...interface{}) (n int, err error) {
	//return fmt.Fprintf(OurStdout, format, a...)
	return
}

func Stack() string {
	buf := make([]byte, 64000)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
