package main

import (
	"fmt"
	"os"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

// next job test
//

func Test001ClientCanSendJobToServer(t *testing.T) {

	cv.Convey("A 'goq serve' processes and a 'goq work' should communicate under the rpc lib", t, func() {

		var jobserv *JobServ
		_ = jobserv
		var err error
		var jobservPid int
		remote := false

		// *** universal test cfg setup
		skipbye := false
		cfg := NewTestConfig(t)
		defer cfg.ByeTestConfig(&skipbye)
		// *** end universal test setup

		cfg.DebugMode = true // reply to badsig packets

		if remote {

			jobservPid, err = NewExternalJobServ(cfg)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n")
			fmt.Printf("[pid %d] spawned a new external JobServ with pid %d\n", os.Getpid(), jobservPid)

		} else {
			jobserv, err = NewJobServ(cfg)
			if err != nil {
				panic(err)
			}
		}

		skip := false
		defer CleanupServer(cfg, jobservPid, jobserv, remote, &skip)
		defer CleanupOutdir(cfg)

		j := NewJob()
		j.Cmd = "bin/good.sh"

		// different cfg, so should be rejected
		sub, err := NewSubmitter(cfg, false)
		if err != nil {
			panic(err)
		}
		reply, _, err := sub.SubmitJobGetReply(j)
		if err != nil {
			panic(err)
		} else {
			cv.So(reply.Msg, cv.ShouldEqual, JOBMSG_ACKSUBMIT)
		}

	})
}
