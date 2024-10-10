package bark

import (
	"fmt"
	"time"
)

var Verbose bool // set to true to debug
var Working bool // currently under investigation

var V = VPrintf
var W = WPrintf

func P(format string, stuff ...interface{}) {
	fmt.Printf("\n "+format+"\n", stuff...)
}

func Q(quietly_ignored ...interface{}) {} // quiet

// get timestamp for logging purposes
func ts() string {
	return time.Now().Format("2006-01-02 15:04:05.999 -0700 MST")
}

// time-stamped printf
func TSPrintf(format string, a ...interface{}) {
	fmt.Printf("%s ", ts())
	fmt.Printf(format, a...)
}

func VPrintf(format string, a ...interface{}) {
	if Verbose {
		TSPrintf(format, a...)
	}
}

func WPrintf(format string, a ...interface{}) {
	if Working {
		TSPrintf(format, a...)
	}
}
