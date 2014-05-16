package main

import (
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// process table generation

var validProcessNumber = regexp.MustCompile(`^[0-9/]+$`)

func IsNumberString(s string) bool {
	match := validProcessNumber.FindStringSubmatch(s)
	if match == nil {
		return false
	}
	return true
}

func init() {
	detectSystem()
}

var Sysname string

func detectSystem() {
	c := exec.Command("uname", "-s")
	o, err := c.Output()

	if err == nil {
		Sysname = strings.Trim(string(o), " \n\t")
	}
}

func ProcessTable() *map[int]bool {
	switch Sysname {
	case "Linux":
		return LinuxPs()

	case "Darwin":
		return DarwinPs()
	}
	m := make(map[int]bool)
	return &m
}

var regexDarwinPs = regexp.MustCompile(`^[^0-9]*[0-9]+[^0-9]+([0-9]+)`)

func DarwinPs() *map[int]bool {

	res := make(map[int]bool)
	o, err := exec.Command("/bin/ps", "-ef").Output()
	if err != nil {
		panic(err)
	}
	s := string(o)

	lines := strings.Split(s, "\n")
	for i := range lines {
		match := regexDarwinPs.FindStringSubmatch(lines[i])
		if match != nil && len(match) == 2 {
			//fmt.Printf("match = %#v\n", match[1])
			num, err := strconv.Atoi(match[1])
			if err == nil {
				res[num] = true
			}
		}
	}
	return &res
}

func LinuxPs() *map[int]bool {

	res := make(map[int]bool)

	fileInfoSlice, err := ioutil.ReadDir("/proc")
	if err != nil {
		panic(err)
	}

	for i := range fileInfoSlice {
		if fileInfoSlice[i].IsDir() {
			if IsNumberString(fileInfoSlice[i].Name()) {
				num, err := strconv.Atoi(fileInfoSlice[i].Name())
				if err != nil {
					panic("IsNumberString regex failed to filter down to only numbers")
				}
				res[num] = true
			}
		}
	}
	return &res
}
