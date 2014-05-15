package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	cv "github.com/smartystreets/goconvey/convey"
)

// worker timeout test
//

func TestWorkerTimeout(t *testing.T) {

	cv.Convey("remotely, over nanomsg, if a goq worker doesn't accept a job after a timeout, the job server should note this", t, func() {
		cv.Convey("and return the job to the waitq to be run by someone else", func() {

			// try to let previous sockets clear out
			//time.Sleep(1000 * time.Millisecond)

			cfg := DefaultCfg()
			// we'll see results much faster if the sender times out faster
			cfg.SendTimeoutMsec = 1000
			//os.Setenv("GOQ_SENDTIMEOUT_MSEC", "1")
			//setSendTimeoutDefaultFromEnv()

			jobserv, err := NewJobServ(cfg.JservAddr, cfg) // use a local jobserv that listens for external worker
			if err != nil {
				panic(err)
			}
			defer CleanupOutdir(cfg)

			fmt.Printf("\n[pid %d] spawned a new local JobServ, listening at '%s'.\n", os.Getpid(), cfg.JservAddr)

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

			// the key difference:
			worker.IsDeaf = true

			worker.SetServer(cfg.JservAddr, cfg)
			_, err = worker.DoOneJob()
			if err != nil {
				// we expect a timeout here, because we are playing deaf and we closed our listening socket.
			}

			// have to poll until everything gets done. Give ourselves 5 seconds.
			timeout := time.After(5 * time.Second)
			var deafcount int

		OuterFor:
			for {
				fmt.Printf("wkto_test: just before blocking on deafcount request.\n")
				select {
				case deafcount = <-jobserv.DeafChan:
					if deafcount > 0 {
						fmt.Printf("wkto_test *success! excellent*: done blocking on deafcount request, deafcount = %d\n", deafcount)
						break OuterFor
					} else {
						fmt.Printf("wkto_test: *ugh, still waiting* done blocking on deafcount request, deafcount = %d\n", deafcount)
					}
				case <-timeout:
					cv.So(deafcount, cv.ShouldEqual, 1)
					fmt.Printf("\nfailing test, no DeafChan 1 after 10 seconds\n")
					break OuterFor
				}
			}

		})
	})
}
