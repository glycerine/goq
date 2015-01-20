package main

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestImmolateAllWorkers(t *testing.T) {

	cv.Convey("When we tell the server 'goq immolateworkers', then the server should request all workers shut themselves (and any jobs they are running) down.", t, func() {
		cv.Convey("and the works should in fact ack this and shutdown.", func() {

			pid := os.Getpid()
			var err error
			remote := false

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			cfg.DebugMode = true // reply to badsig packets

			jobserv, jobservPid := HelperNewJobServ(cfg, remote)

			defer CleanupServer(cfg, jobservPid, jobserv, remote, nil)
			defer CleanupOutdir(cfg)

			nwork := 10
			wks := make([]<-chan bool, nwork)
			for i := 0; i < nwork; i++ {
				w := HelperNewWorkerMonitored(cfg)
				afterSend, afterReply := w.NS.MonitorSend, w.NR.MonitorRecv
				wks[i] = afterReply
				go func(w *Worker) {
					w.DoOneJob() // _, err = w.DoOneJob() // makes a data race on err.
					w.Destroy()  // -race reports error here. Bug: http://code.google.com/p/go/issues/detail?id=8053
				}(w)
				<-afterSend
			}

			// We should see nwork workers
			snapmap := HelperSnapmap(cfg)

			ww, ok := snapmap["waitingWorkers"]
			if !ok {
				panic(fmt.Sprintf("server stat must include waitingWorkers"))
			}
			iww, err := strconv.Atoi(ww)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n    We should see %d waiting workers now, and we see: %d. snapmap: %#v\n", nwork, iww, snapmap)
			cv.So(iww, cv.ShouldEqual, nwork)

			// now immo
			sub, err := NewSubmitter(GenAddress(), cfg, false)
			if err != nil {
				panic(err)
			}

			err = sub.SubmitImmoJob()
			if err != nil {
				fmt.Printf("[pid %d] error while submitting ImmolateWorkers command to server '%s': %s\n", pid, cfg.JservAddr(), err)
				os.Exit(1)
			}

			fmt.Printf("[pid %d] immolate workers command submitted to server '%s':\n", pid, cfg.JservAddr())

			// wait for all workers to receive replies
			for i := 0; i < nwork; i++ {
				<-wks[i]
			}

			// check snap again
			snapmap2 := HelperSnapmap(cfg)

			ww2, ok := snapmap2["waitingWorkers"]
			if !ok {
				panic(fmt.Sprintf("server stat must include waitingWorkers"))
			}
			iww2, err := strconv.Atoi(ww2)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n    We should see 0 waiting workers now, and we see: %d.  snapmap2: %#v\n", iww2, snapmap2)
			cv.So(iww2, cv.ShouldEqual, 0)

		})
	})
}
