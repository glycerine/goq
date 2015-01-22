package main

import (
	"fmt"
	"os"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

// open-file-handles test
//
// we are seeing goq server max out at 500 jobs submitted, because it
// is holding more than 1024 open file handles. So we need to be
// more frugal and keep all file handles closed when not in use.

func TestSubmitDoesNotLeaveFileHandlesOpen001(t *testing.T) {

	cv.Convey("job submits should not result in extra file handles being open on the server", t, func() {

		// allow all child processes to communicate

		// *** universal test cfg setup
		skipbye := false
		cfg := NewTestConfig()
		defer cfg.ByeTestConfig(&skipbye)
		// *** end universal test setup

		// ensure any previous test has released our port before proceeding.
		WaitUntilAddrAvailable(cfg.JservAddr())

		cfg.DebugMode = true
		childpid, err := NewExternalJobServ(cfg)
		if err != nil {
			panic(err)
		}
		fmt.Printf("[pid %d] spawned a new external JobServ with pid %d\n", os.Getpid(), childpid)

		startingOFH := OpenFiles(childpid)

		j := NewJob()
		j.Cmd = "bin/good.sh"

		sub, err := NewSubmitter(GenAddress(), cfg, false)
		if err != nil {
			panic(err)
		}
		sub.SubmitJob(j)

		endingOFH := OpenFiles(childpid)

		// *important* cleanup, and wait for cleanup to finish, so the next test can run.
		// has no Fromaddr, so crashes: SendShutdown(cfg.JservAddr, cfg)
		sub.SubmitShutdownJob()

		WaitForShutdownWithTimeout(childpid)

		if len(endingOFH) != len(startingOFH) {
			fmt.Printf("\n\n ending minus starting : \n")
			ShowStrings(SetDiff(endingOFH, startingOFH))
			fmt.Printf("\n\n starting minus ending : \n")
			ShowStrings(SetDiff(startingOFH, endingOFH))
		}

		cv.So(len(endingOFH), cv.ShouldEqual, len(startingOFH))

	})
}
