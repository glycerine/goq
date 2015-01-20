package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"fmt"
	"strconv"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestSaveServerState(t *testing.T) {

	cv.Convey("Given a server with running jobs", t, func() {
		cv.Convey("if the server is killed, upon resumption it should recall the already running jobs", func() {

			remote := false

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			//cfg.SendTimeoutMsec = 5000
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			cfg.DebugMode = true // reply to badsig packets

			jobserv, jobservPid := HelperNewJobServ(cfg, remote)
			skip := false
			defer CleanupServer(cfg, jobservPid, jobserv, remote, &skip)
			defer CleanupOutdir(cfg)

			// a job that will go forever unless cancelled
			j := NewJob()
			j.Cmd = "bin/forev.sh"

			_, job := HelperSubJobGetReply(j, cfg)
			fmt.Printf("restore-state-test: forev job started: Aboutjid: %d, job: %s\n", job.Aboutjid, job)

			_, job2 := HelperSubJobGetReply(j, cfg)
			fmt.Printf("restore-state-test: 2nd forev job started: Aboutjid: %d, job: %s\n", job2.Aboutjid, job2)

			// start a worker to start on the first job
			w := HelperNewWorkerMonitored(cfg)
			defer func() {
				fmt.Printf("\ndeffered w.Destroy running.\n")
				w.Destroy()
			}()
			w.AttemptOnlyOneJob()

			// make sure worker gets the job before shutting down server
			<-w.NS.MonitorSend
			fmt.Printf("\n  restore-state-test got past MonitorSend\n")
			<-w.NR.MonitorRecv
			fmt.Printf("\n  restore-state-test got past MonitorRecv\n")

			<-w.MonitorShepJobStart
			fmt.Printf("\n  restore-state-test got past MonitorShepJobStart\n")

			// snapshot should show one running job
			confirmOneJobRunningOneWaiting(cfg)

			// shutdown the server
			jobserv.Ctrl <- die
			<-jobserv.Done
			skip = true // no need to cleanup jobserv now.

			// and then restart it, to see if it recovers its state
			jobserv2, jobservPid2 := HelperNewJobServ(cfg, remote)
			defer CleanupServer(cfg, jobservPid2, jobserv2, remote, nil)

			// snapshot should show one running job
			confirmOneJobRunningOneWaiting(cfg)

		})
	})
}

func confirmOneJobRunningOneWaiting(cfg *Config) {
	snapmap := HelperSnapmap(cfg)
	running, ok := snapmap["runQlen"]
	if !ok {
		panic(fmt.Sprintf("server stat must include runQlen"))
	}
	irunning, err := strconv.Atoi(running)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n    We should see one running job now, and we see: %d. snapmap: %#v\n", irunning, snapmap)
	cv.So(irunning, cv.ShouldEqual, 1)

	waiting, ok := snapmap["waitingJobs"]
	if !ok {
		panic(fmt.Sprintf("server stat must include waitingJobs"))
	}
	iwaiting, err := strconv.Atoi(waiting)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n    We should see one waiting job now, and we see: %d. snapmap: %#v\n", iwaiting, snapmap)
	cv.So(iwaiting, cv.ShouldEqual, 1)

}
