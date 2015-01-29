package main

import (
	"fmt"
	"syscall"
)

func ShowRlimit() {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		panic(fmt.Sprintf("Error Getting Rlimit '%s'", err))
	}
	fmt.Printf("starting rlimit.Cur = %d, rlimit.Max = %d\n", rLimit.Cur, rLimit.Max)
}

func SetRlimit() {
	var rLimit syscall.Rlimit
	rLimit.Max = 999999
	rLimit.Cur = 999999
	err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		panic(fmt.Sprintf("Error Setting Rlimit '%s'", err))
	}
	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		panic(fmt.Sprintf("Error Getting Rlimit '%s'", err))
	}
	fmt.Printf("final: rlimit.Cur = %d, rlimit.Max = %d\n", rLimit.Cur, rLimit.Max)
}
