package main

import (
	"fmt"
	"strings"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestSheparding(t *testing.T) {

	// *** universal test cfg setup
	skipbye := false
	cfg := NewTestConfig()
	defer cfg.ByeTestConfig(&skipbye)
	// *** end universal test setup

	w := HelperNewWorkerDontStart(cfg)

	// Only pass t into top-level Convey calls
	cv.Convey("goq should be able to shepard a shell process", t, func() {

		cv.Convey("then stdout/stderr should be returned, without crashing the shepard ", func() {

			j := NewJob()
			j.Cmd = "./bin/badboy.sh"
			j.Workeraddr = w.Addr
			j.Serveraddr = w.ServerAddr
			w.Shepard(j)
			<-w.ShepSaysJobStarted
			j = <-w.ShepSaysJobDone

			fmt.Printf("\n\n j.Out = %#v\n", j.Out)
			cv.So(len(j.Out), cv.ShouldEqual, 2)
			cv.So(j.Out[0], cv.ShouldEqual, "some stderr")
			cv.So(j.Out[1], cv.ShouldEqual, "some stdout")
		})

		cv.Convey("then segfaulting process should be handled ", func() {

			j := NewJob()
			j.Cmd = "./bin/faulter"
			w.Shepard(j)
			<-w.ShepSaysJobStarted
			j = <-w.ShepSaysJobDone

			cv.So(len(j.Out), cv.ShouldEqual, 2)
			expected := "Shepard finds non-nil err on trying to Wait() on cmd './bin/faulter' in dir ''"
			cv.So(j.Out[1][:len(expected)], cv.ShouldEqual, expected)
		})

		cv.Convey("then executable file not found errors should be handled ", func() {
			j := NewJob()
			j.Cmd = "./does-not-exist"
			w.Shepard(j)
			<-w.ShepSaysJobStarted
			j = <-w.ShepSaysJobDone

			cv.So(len(j.Out), cv.ShouldEqual, 2)
			//cv.So(j.Out[0], cv.ShouldEqual, "")
			expectedSuffix := `Shepard finds non-nil err on trying to Wait() on cmd './does-not-exist' in dir ''`
			cv.So(strings.HasPrefix(j.Out[1], expectedSuffix), cv.ShouldEqual, true)
		})

	})
}
