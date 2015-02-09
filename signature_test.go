package main

import (
	"fmt"
	"os"
	"testing"

	schema "github.com/glycerine/goq/schema"
	cv "github.com/glycerine/goconvey/convey"
)

// signature test
//

func TestSignatureConsistent(t *testing.T) {

	cv.Convey("signing a job with the same clusterid as you check with should be consistent, even after reserialization.", t, func() {
		job := MakeTestJob()
		cfg := &Config{
			ClusterId: RandomClusterId(),
		}
		SignJob(job, cfg)
		cv.So(JobSignatureOkay(job, cfg), cv.ShouldEqual, true)

		// then pass through capn serial/deserialize
		buf, _ := JobToCapnp(job)
		job2 := CapnpToJob(&buf)

		cv.So(JobSignatureOkay(job2, cfg), cv.ShouldEqual, true)
		cv.So(GetJobSignature(job2, cfg), cv.ShouldEqual, GetJobSignature(job, cfg))

	})
}

func TestSubmitBadSignatureDetected(t *testing.T) {

	cv.Convey("When we submit a job or workready to the server signed by a non-matching signature", t, func() {
		cv.Convey("then the server should reject those requests and keep stats on them", func() {

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
			defer CleanupServer(cfg, jobservPid, jobserv, remote, nil)
			defer CleanupOutdir(cfg)

			//diffCfg := DefaultCfg() // can't do this because its on 1776, not 2779 for testing.
			diffCfg := CopyConfig(cfg) // keep same aes so they can communicate.
			diffCfg.ClusterId = GetRandomCidDistinctFrom(cfg.ClusterId)
			//diffCfg.SendTimeoutMsec = 30000

			j := NewJob()
			j.Cmd = "bin/good.sh"

			// different cfg, so should be rejected
			sub, err := NewSubmitter(GenAddress(), diffCfg, false)
			if err != nil {
				panic(err)
			}
			reply, err := sub.SubmitJobGetReply(j)
			if err != nil {
				fmt.Printf("ignoring likely timeout error: %#v\n", err)
			} else {
				cv.So(reply.Msg, cv.ShouldEqual, schema.JOBMSG_REJECTBADSIG)
			}

			// different cf, so worker should be rejected too.
			w := HelperNewWorkerMonitored(diffCfg)
			w.AttemptOnlyOneJob()

			// this job won't be successful, because of the bad sig, so we'll
			// block, waiting for a good job to finish. Hmm... solution:
			// monitor, and insist that
			// we get past send and recv.
			<-w.NS.MonitorSend
			<-w.NR.MonitorRecv

			// now should be safe to cleanup
			w.Destroy()

			// We should see one worker and one submit reject in the server stats
			serverSnap, err := SubmitGetServerSnapshot(cfg)
			if err != nil {
				panic(err)
			}
			snapmap := EnvToMap(serverSnap)
			fmt.Printf("serverSnap = %#v\n", serverSnap)

			cv.So(len(snapmap), cv.ShouldBeGreaterThan, 8)
			cv.So(snapmap["droppedBadSigCount"], cv.ShouldEqual, "2")

		})
	})
}
