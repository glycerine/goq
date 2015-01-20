package main

import (
	"os"
	"os/exec"
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
		by, err := c.CombinedOutput()
		if err != nil {
			panic(err)
		}

		cv.So(string(by), cv.ShouldEqual, `hello
`)
	})
}
