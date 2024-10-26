package main

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestShellOutShouldRunAndRedirect999(t *testing.T) {

	cv.Convey(`Given a command with a redirect, such as "/bin/echo 'hello > hi.there", then MakeShellScript() should put it in a file and make it executable`, t, func() {
		path, err := MakeShellScript("/bin/echo", []string{"hello", ">", "hi.there;", "cat", "hi.there"}, ".")

		defer os.Remove(path)
		defer os.Remove("hi.there")

		cv.So(err, cv.ShouldEqual, nil)
		cv.So(path, cv.ShouldNotEqual, "")

		c := exec.Command(path)

		// This matches what shep does at the moment. see shep.go:104
		// On windows under cygwin, os.Command is not happy running #!/bin/bash
		// shell scripts. Without the following special case, it complains:
		//   Line 25: - fork/exec .\.goq.run.script_814171736: %1 is not a valid Win32 application.
		if runtime.GOOS == "windows" {
			// TODO: don't hard code bash location
			c = exec.Command("C:\\cygwin64\\bin\\bash.exe", path)
		}

		by, err := c.CombinedOutput()
		if err != nil {
			panic(err)
		}

		cv.So(string(by), cv.ShouldEqual, `hello
`)
	})
}
