package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestServerLocFileReadWrite(t *testing.T) {

	cv.Convey("WriteServerLoc() should write to GOQ_HOME/serverloc, and ReadServerLoc() should read that info back", t, func() {
		skipbye := false
		cfg := NewTestConfig()
		defer cfg.ByeTestConfig(&skipbye)

		//skipbye = true
		fn := ServerLocFile(cfg)
		WriteServerLoc(cfg)
		exists := FileExists(fn)
		cv.So(exists, cv.ShouldEqual, true)

		sl, err := ioutil.ReadFile(fn)
		if err != nil {
			panic(err)
		}

		lines := strings.Split(string(sl), "\n")
		expect0 := `export GOQ_JSERV_IP=`
		cv.So(lines[0][:len(expect0)], cv.ShouldEqual, expect0)
		cv.So(lines[1], cv.ShouldEqual, `export GOQ_JSERV_PORT=2776`)
		cv.So(lines[2], cv.ShouldEqual, `export GOQ_SENDTIMEOUT_MSEC=1000`)
		cv.So(lines[3], cv.ShouldEqual, `export GOQ_RECVTIMEOUT_MSEC=1000`)
		cv.So(lines[4], cv.ShouldEqual, `export GOQ_HEARTBEAT_SEC=5`)

		// fill cfg with some test garbage.
		orig := *cfg
		cfg.SendTimeoutMsec = -1
		cfg.JservIP = "0.0.0.0"
		cfg.JservPort = 9999
		cfg.Heartbeat = -343
		cv.So(cfg.SendTimeoutMsec, cv.ShouldNotEqual, orig.SendTimeoutMsec)
		cv.So(cfg.JservIP, cv.ShouldNotEqual, orig.JservIP)
		cv.So(cfg.JservPort, cv.ShouldNotEqual, orig.JservPort)
		cv.So(cfg.Heartbeat, cv.ShouldNotEqual, orig.Heartbeat)

		ReadServerLoc(cfg)
		fmt.Printf("\n    After reading our save serverloc info from file, the restored info should obliterate the test garbage we filled in.\n")
		cv.So(cfg.SendTimeoutMsec, cv.ShouldEqual, orig.SendTimeoutMsec)
		cv.So(cfg.JservIP, cv.ShouldEqual, orig.JservIP)
		cv.So(cfg.JservPort, cv.ShouldEqual, orig.JservPort)
		cv.So(cfg.Heartbeat, cv.ShouldEqual, orig.Heartbeat)

		// didn't start a server, so don't need this:
		//CleanupOutdir(cfg)
		//CleanupServer(cfg, jobservPid, jobserv, remote, nil)

	})
}

func TestServerLocFileControlsServerPort(t *testing.T) {

	cv.Convey("The GOQ_HOME/serverloc setting for GOQ_JSERV_PORT should take affect when we start a server", t, func() {

		var jobserv *JobServ
		var err error
		var jobservPid int
		remote := false

		skipbye := false
		cfg := NewTestConfig()
		defer cfg.ByeTestConfig(&skipbye)

		//skipbye = true
		fn := ServerLocFile(cfg)

		newPort := 2779
		cfg.JservPort = newPort
		fmt.Printf("  When we try to start a jobserver on port %d, aftering writing that to .goq/serverloc, the server should start on that port.\n", newPort)
		WriteServerLoc(cfg)
		exists := FileExists(fn)
		cv.So(exists, cv.ShouldEqual, true)

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
			fmt.Printf("\n jobserv.Cfg.JservPort = %d\n", jobserv.Cfg.JservPort)
			fmt.Printf("\n jobserv.Addr = %s\n", jobserv.Addr)
			cv.So(jobserv.Cfg.JservPort, cv.ShouldEqual, newPort)
		}
		CleanupOutdir(cfg)
		CleanupServer(cfg, jobservPid, jobserv, remote, nil)
		WaitUntilAddrAvailable(cfg.JservAddr())
		cfg.JservPort = 2776
		WaitUntilAddrAvailable(cfg.JservAddr())

	})
}
