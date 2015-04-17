// +build linux

package main

func netstat_commandline() (string, []string) {
	return "netstat", []string{"-nuptl"}
}
