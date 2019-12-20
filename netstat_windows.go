// +build windows

package main

func netstat_commandline() (string, []string) {
	return "netstat", []string{"-an"}
}
