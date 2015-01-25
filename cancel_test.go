package main

import (
	"fmt"
	"strconv"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestCancelJobInProgress(t *testing.T) {

	cv.Convey("When we ask workers to cancel one job or all jobs in progress,", t, func() {
		cv.Convey("they should stay alive but kill their sheparded processes and report back to server.", func() {

			//pid := os.Getpid()
			var err error
			remote := false

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			//cfg.SendTimeoutMsec = 5000
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			cfg.DebugMode = true // reply to badsig packets

			jobserv, jobservPid := HelperNewJobServ(cfg, remote)

			defer CleanupServer(cfg, jobservPid, jobserv, remote, nil)
			defer CleanupOutdir(cfg)

			// a job that will go forever unless cancelled
			j := NewJob()
			j.Cmd = "bin/forev.sh"

			_, job := HelperSubJobGetReply(j, cfg)
			fmt.Printf("cancel-test: forev job started: Aboutjid: %d, job: %s\n", job.Aboutjid, job)

			// the forev job won't be successful, because it sleeps forever.

			// start a (local inproc) worker to do that job
			w := HelperNewWorkerMonitored(cfg)
			/*
				defer func() {
					fmt.Printf("\ndeffered w.Destroy running.\n")
					w.Destroy()
				}()
			*/
			w.AttemptOnlyOneJob()

			// make sure worker gets the job before trying to cancel
			<-w.NS.MonitorSend
			fmt.Printf("\n  cancel-test got past MonitorSend\n")
			<-w.NR.MonitorRecv
			fmt.Printf("\n  cancel-test got past MonitorRecv\n")

			<-w.MonitorShepJobStart
			fmt.Printf("\n  cancel-test got past MonitorShepJobStart\n")

			// send the cancel
			sub, err := NewSubmitter(GenAddress(), cfg, false)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n  cancel-test got past NewSubmitter()\n")

			err = sub.SubmitCancelJob(job.Aboutjid)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n  cancel-test got past SubmitCancelJob\n")

			<-w.MonitorShepJobDone
			fmt.Printf("\n  cancel-test got past MonitorShepJobDone\n")

			// At this point, there is still a race to get to the jobdone msg with .Cancelled set
			// back to the server before we query stats. So tell server to signal after first
			// Cancelled job is received.

			<-jobserv.FirstCancelDone
			fmt.Printf("\n  cancel-test got past <-jobserv.FirstCancelDone\n")

			// We should see nwork workers
			snapmap := HelperSnapmap(cfg)

			canned, ok := snapmap["cancelledJobCount"]
			if !ok {
				panic(fmt.Sprintf("server stat must include cancelledJobCount"))
			}
			ican, err := strconv.Atoi(canned)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n    We should see one cancelled job now, and we see: %d. snapmap: %#v\n", ican, snapmap)
			cv.So(ican, cv.ShouldEqual, 1)

			w.Destroy()
		})
	})
}
