package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
)

func TestSubmitRemote(t *testing.T) {

	cv.Convey("remotely, over nanomsg, goq should be able to submit job to the server", t, func() {
		cv.Convey("and then server should get back the expected output from the job run", func() {

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

			j := NewJob()
			j.Cmd = "bin/good.sh"

			sub, err := NewSubmitter(GenAddress(), cfg, false)
			if err != nil {
				panic(err)
			}
			sub.SubmitJob(j)

			worker, err := NewWorker(GenAddress(), cfg, nil)
			if err != nil {
				panic(err)
			}
			jobout, err := worker.DoOneJob()
			if err != nil {
				panic(err)
			}
			worker.Destroy()

			// *important* cleanup, and wait for cleanup to finish, so the next test can run.
			// has no Fromaddr, so crashes: SendShutdown(cfg.JservAddr, cfg)
			sub.SubmitShutdownJob()

			WaitForShutdownWithTimeout(childpid, cfg)

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
				fmt.Printf("\n Problem starting external job server! Is goq on your path? It must be. Do 'go install' first, then re-run this test.")
				panic(err)
			}
			fmt.Printf("\n[pid %d] spawned a new external JobServ with pid %d\n", os.Getpid(), jobservPid)

			// wait until process shows up in /proc
			t0 := time.Now()
			waited := 0
			for {
				pt := *ProcessTable()
				_, jsAlive := pt[jobservPid]
				if jsAlive {
					break
				}
				time.Sleep(50 * time.Millisecond)
				waited++
				VPrintf("cfg.SendTimeoutMsec = %v\n", cfg.SendTimeoutMsec)
				if time.Since(t0) > time.Millisecond*time.Duration(cfg.SendTimeoutMsec)*3 {
					panic(fmt.Sprintf("jobserv with expected pid %d did not show up in /proc after 3 recv timeouts", jobservPid))
				}
			}
			fmt.Printf("\njobserv with expected pid %d was *found* in /proc after %d waits of 50msec\n", jobservPid, waited)

			// then kill it
			SendShutdown(cfg)

			fmt.Printf("\nsent shutdown request\n")

			// verify kill
			// non-deterministic, but try to give them time to be gone.
			WaitForShutdownWithTimeout(jobservPid, cfg)

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

			cfg.JservIP = ""
			jobserv, err := NewJobServ(cfg)
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

func WaitForShutdownWithTimeout(jobservPid int, cfg *Config) {
	time.Sleep(100 * time.Millisecond)
	waited := 0
	t0 := time.Now()
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
		if time.Since(t0) > time.Millisecond*time.Duration(cfg.SendTimeoutMsec)*3 {
			fmt.Printf("failed to exit: dumping the goroutines on the server to see where we are stuck.\n")
			syscall.Kill(jobservPid, syscall.SIGQUIT)
			time.Sleep(4 * time.Second)
			panic(fmt.Sprintf("jobserv with expected pid %d did not disappear from /proc after 3 timeouts", jobservPid))
		}
	}
}
