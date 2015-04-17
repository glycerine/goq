// +build darwin

package main

func netstat_commandline() (string, []string) {
	return "lsof", []string{"-iTCP", "-n", "-P"}
}
