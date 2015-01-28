package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

var scriptPrefix = ".goq.run.script_"

// MakeShellScript makes a bash script with the cmd and args in it.
// PRE: dir exists
func MakeShellScript(cmd string, args []string, dir string) (pathToScript string, err error) {

	//fmt.Printf("\n MakeShellScript called with cmd: '%v', args: '%v', dir: '%v'\n", cmd, args, dir)

	file, err := ioutil.TempFile(dir, scriptPrefix)
	checkError(err)

	var fullPath string
	if dir == "" {
		fullPath = file.Name()
	} else {
		fullPath = dir + "/" + path.Base(file.Name())
	}
	//fmt.Printf("fullPath = '%v'\n", fullPath)

	_, err = file.WriteString("#!/bin/bash\n")
	checkError(err)

	_, err = file.WriteString(cmd)
	checkError(err)

	_, err = file.WriteString(" ")
	checkError(err)

	_, err = file.WriteString(strings.Join(args, " "))
	checkError(err)

	err = file.Close()
	checkError(err)

	err = os.Chmod(fullPath, 0755)

	//fmt.Printf("\n MakeShellScript returning '%v'\n", fullPath)
	return fullPath, err
}
