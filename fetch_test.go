package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"fmt"
	"os"
	"testing"
	"time"

	cv "github.com/smartystreets/goconvey/convey"
)

func TestFetchingJobLocal(t *testing.T) {

	cv.Convey("goq shep should be able fetch a job to run from the server", t, func() {
		cv.Convey("and then server should get back the expected output from the job run", func() {

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			addr_use := "" // implies stay all in local goroutines

			jserv, err := NewJobServ(addr_use, cfg)
			if err != nil {
				panic(err)
			}
			defer CleanupOutdir(cfg)
			defer CleanupServer(cfg, -1, jserv, false, nil)

			j := NewJob()
			j.Cmd = "bin/good.sh"
			j.IsLocal = true

			fmt.Printf("\n TestFetchingJobLocal doing a local jserv.SubmitJob(j)\n")
			jserv.SubmitJob(j)

			worker, err := NewLocalWorker(jserv)
			if err != nil {
				panic(err)
			}
			jobout, err := worker.DoOneJob()
			if err != nil {
				panic(err)
			}

			cv.So(len(jobout.Out), cv.ShouldEqual, 2)
			cv.So(jobout.Out[0], cv.ShouldEqual, "I'm starting some work")
			cv.So(jobout.Out[1], cv.ShouldEqual, "I'm done with my work")

		})
	})
}

func TestSubmitLocal(t *testing.T) {

	cv.Convey("goq should be able to submit job to the server", t, func() {
		cv.Convey("and then server should get back the expected output from the job run", func() {

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			//addr_use := JSERV_ADDR
			addr_use := "" // implies stay all in local goroutines

			jserv, err := NewJobServ(addr_use, cfg)
			if err != nil {
				panic(err)
			}
			defer CleanupOutdir(cfg)
			defer CleanupServer(cfg, -1, jserv, false, nil)

			j := NewJob()
			j.Cmd = "bin/good.sh"
			j.IsLocal = true

			sub, err := NewLocalSubmitter(jserv)
			if err != nil {
				panic(err)
			}
			sub.SubmitJob(j)

			worker, err := NewLocalWorker(jserv)
			if err != nil {
				panic(err)
			}
			jobout, err := worker.DoOneJob()
			if err != nil {
				panic(err)
			}

			cv.So(len(jobout.Out), cv.ShouldEqual, 2)
			cv.So(jobout.Out[0], cv.ShouldEqual, "I'm starting some work")
			cv.So(jobout.Out[1], cv.ShouldEqual, "I'm done with my work")

		})
	})
}

func TestSubmitRemote(t *testing.T) {

	cv.Convey("remotely, over nanomsg, goq should be able to submit job to the server", t, func() {
		cv.Convey("and then server should get back the expected output from the job run", func() {

			// allow all child processes to communicate

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			WaitUntilAddrAvailable(cfg.JservAddr)

			cfg.DebugMode = true
			childpid, err := NewExternalJobServ(cfg)
			if err != nil {
				panic(err)
			}
			fmt.Printf("[pid %d] spawned a new external JobServ with pid %d\n", os.Getpid(), childpid)

			j := NewJob()
			j.Cmd = "bin/good.sh"

			sub, err := NewSubmitter(GenAddress(), cfg, false)
			if err != nil {
				panic(err)
			}
			sub.SubmitJob(j)

			worker, err := NewWorker(GenAddress(), cfg)
			if err != nil {
				panic(err)
			}
			worker.SetServer(cfg.JservAddr, cfg)
			jobout, err := worker.DoOneJob()
			if err != nil {
				panic(err)
			}

			// *important* cleanup, and wait for cleanup to finish, so the next test can run.
			// has no Fromaddr, so crashes: SendShutdown(cfg.JservAddr, cfg)
			sub.SubmitShutdownJob()

			WaitForShutdownWithTimeout(childpid)

			cv.So(len(jobout.Out), cv.ShouldEqual, 2)
			cv.So(jobout.Out[0], cv.ShouldEqual, "I'm starting some work")
			cv.So(jobout.Out[1], cv.ShouldEqual, "I'm done with my work")

		})
	})
}

func TestSubmitShutdownToRemoteJobServ(t *testing.T) {

	cv.Convey("remotely, over nanomsg, goq should be able to submit a shutdown job to the server", t, func() {
		cv.Convey("and then server process should shut itself down cleanly", func() {

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			jobservPid, err := NewExternalJobServ(cfg)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n[pid %d] spawned a new external JobServ with pid %d\n", os.Getpid(), jobservPid)

			// wait until process shows up in /proc
			waited := 0
			for {
				pt := *ProcessTable()
				_, jsAlive := pt[jobservPid]
				if jsAlive {
					break
				}
				time.Sleep(50 * time.Millisecond)
				waited++
				if waited > 10 {
					panic(fmt.Sprintf("jobserv with expected pid %d did not show up in /proc after 10 waits", jobservPid))
				}
			}
			fmt.Printf("\njobserv with expected pid %d was *found* in /proc after %d waits of 50msec\n", jobservPid, waited)

			// then kill it
			SendShutdown(cfg)

			fmt.Printf("\nsent shutdown request\n")

			// verify kill
			// non-deterministic, but try to give them time to be gone.
			WaitForShutdownWithTimeout(jobservPid)

			pt := *ProcessTable()
			_, jsAlive := pt[jobservPid]
			if jsAlive == false {
				fmt.Printf("jobserv at pid %d appears to have been shutdown by our comand.\n", jobservPid)
			}
			cv.So(jsAlive, cv.ShouldEqual, false)
		})
	})
}

func TestSubmitShutdownToLocalJobServ(t *testing.T) {

	cv.Convey("with a local jobserv, we should be able to submit a shutdown job to the server", t, func() {
		cv.Convey("and then server go routine should shut itself down cleanly", func() {

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			jobserv, err := NewJobServ("", cfg)
			if err != nil {
				panic(err)
			}
			defer CleanupOutdir(cfg)

			sub, err := NewLocalSubmitter(jobserv)
			if err != nil {
				panic(err)
			}
			sub.SubmitShutdownJob()

			<-jobserv.Done
			// we should get past the receive on Done when jobserv closes down:
			cv.So(true, cv.ShouldEqual, true)

		})
	})
}

func WaitForShutdownWithTimeout(jobservPid int) {
	time.Sleep(50 * time.Millisecond)
	waited := 0
	for {
		pt := *ProcessTable()
		//fmt.Printf("pt = %#v\n", pt)
		_, jsAlive := pt[jobservPid]
		if !jsAlive {
			break
		}
		fmt.Printf("jobserv at pid %d is still alive...\n", jobservPid)
		time.Sleep(100 * time.Millisecond)
		waited++
		if waited > 10 {
			panic(fmt.Sprintf("jobserv with expected pid %d did not disappear from /proc after 10 waits", jobservPid))
		}
	}
}
