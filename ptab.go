package main

import (
	"io/ioutil"
	"regexp"
	"strconv"
)

var validProcessNumber = regexp.MustCompile(`^[0-9/]+$`)

func IsNumberString(s string) bool {
	match := validProcessNumber.FindStringSubmatch(s)
	if match == nil {
		return false
	}
	return true
}

func ProcessTable() map[int]bool {
	fileInfoSlice, err := ioutil.ReadDir("/proc")
	if err != nil {
		panic(err)
	}
	res := make(map[int]bool)

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
	return res
}
