package main

import (
	"fmt"
	"os"
	"testing"

	schema "github.com/glycerine/goq/schema"
	cv "github.com/glycerine/goconvey/convey"
)

// next job test
//

func TestNextJobPersisted(t *testing.T) {

	cv.Convey("Two serial 'goq serve' processes should use non-overlapping job identifiers", t, func() {
		cv.Convey("by persisting the next-to-use to disk, the next 'goq serve' should start where the previous left off.", func() {

			var jobserv *JobServ
			var err error
			var jobservPid int
			remote := false

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
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
			//defer CleanupServer(cfg, jobservPid, jobserv, remote, &skipbye)
			//defer CleanupOutdir(cfg)

			j := NewJob()
			j.Cmd = "bin/good.sh"

			// different cfg, so should be rejected
			sub, err := NewSubmitter(GenAddress(), cfg, false)
			if err != nil {
				panic(err)
			}
			reply, err := sub.SubmitJobGetReply(j)
			if err != nil {
				panic(err)
			} else {
				cv.So(reply.Msg, cv.ShouldEqual, schema.JOBMSG_ACKSUBMIT)
			}

			// we should see one job done, and nextJobId of 2
			serverSnap, err := SubmitGetServerSnapshot(cfg)
			if err != nil {
				panic(err)
			}
			snapmap := EnvToMap(serverSnap)
			fmt.Printf("serverSnap = %#v\n", serverSnap)

			cv.So(snapmap["nextJobId"], cv.ShouldEqual, "2")

			// now kill the old server, and start a new one
			CleanupServer(cfg, jobservPid, jobserv, remote, nil)

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
			defer CleanupServer(cfg, jobservPid, jobserv, remote, &skipbye)
			defer CleanupOutdir(cfg)

			// we should see nextJobId of 2, not 1.
			serverSnap, err = SubmitGetServerSnapshot(cfg)
			if err != nil {
				panic(err)
			}
			snapmap = EnvToMap(serverSnap)
			fmt.Printf("serverSnap = %#v\n", serverSnap)

			cv.So(len(snapmap), cv.ShouldBeGreaterThan, 8)
			cv.So(snapmap["nextJobId"], cv.ShouldEqual, "2")

		})
	})
}
