// +build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
)

// windows specific implementations/stubs.

var _ = fmt.Printf
var _ = syscall.EscapeArg

func ShowRlimit() {
	// var rLimit syscall.Rlimit
	// err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error Getting Rlimit '%s'", err))
	// }
	// fmt.Printf("starting rlimit.Cur = %d, rlimit.Max = %d\n", rLimit.Cur, rLimit.Max)
}

func SetRlimit() {
	// var rLimit syscall.Rlimit
	// rLimit.Max = 999999
	// rLimit.Cur = 999999
	// err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error Setting Rlimit '%s'", err))
	// }
	// err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error Getting Rlimit '%s'", err))
	// }
	// fmt.Printf("final: rlimit.Cur = %d, rlimit.Max = %d\n", rLimit.Cur, rLimit.Max)
}

func systemCallSetGroup(c *exec.Cmd) {
	//c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessGroup(pid int) {
	// // try to kill via PGID; we ran this child in its own process group for this.
	// pgid, pgidErr := syscall.Getpgid(pid)

	// proc, err := os.FindProcess(w.Pid)
	// _ = err // ignored. possible race; might already be gone.
	// if pgidErr == nil {
	// 	syscall.Kill(-pgid, 9) // note the minus sign
	// }
}
