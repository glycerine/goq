package main

import (
	"testing"

	cv "github.com/smartystreets/goconvey/convey"
)

func TestSheparding(t *testing.T) {

	// Only pass t into top-level Convey calls
	cv.Convey("goq should be able to shepard a shell process", t, func() {
		cv.Convey("when the sheparded process returns an error and output on stdout/stderr", func() {
			cv.Convey("then stdout/stderr should be returned, without crashing the shepard ", func() {
				out, _ := Shepard("", "./bin/badboy.sh", []string{})
				//fmt.Printf("\n\nout = %#v\n", out)
				cv.So(len(out), cv.ShouldEqual, 2)
				cv.So(out[0], cv.ShouldEqual, "some stderr")
				cv.So(out[1], cv.ShouldEqual, "some stdout")
			})

			cv.Convey("then segfaulting process should be handled ", func() {
				out, _ := Shepard("", "./bin/faulter", []string{})
				//fmt.Printf("\n\nout = %#v\n", out)
				cv.So(len(out), cv.ShouldEqual, 2)
				cv.So(out[0], cv.ShouldEqual, "")
				expected := "Shepard finds non-nil err on running cmd './bin/faulter' in dir '': signal: segmentation fault"
				cv.So(out[1][:len(expected)], cv.ShouldEqual, expected)
			})

			cv.Convey("then executable file not found errors should be handled ", func() {
				out, _ := Shepard("", "./does-not-exist", []string{})
				//fmt.Printf("\n\nout = %#v\n", out)
				cv.So(len(out), cv.ShouldEqual, 2)
				cv.So(out[0], cv.ShouldEqual, "")
				cv.So(out[1], cv.ShouldEqual, `Shepard finds non-nil err on running cmd './does-not-exist' in dir '': exec: "./does-not-exist": stat ./does-not-exist: no such file or directory`)
			})

		})
	})
}
