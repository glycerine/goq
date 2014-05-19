package main

import (
	"fmt"
	"io/ioutil"
	"testing"

	cv "github.com/smartystreets/goconvey/convey"
)

func TestServerLocFileReadWrite(t *testing.T) {

	cv.Convey("WriteServerLoc() should write to $GOQ_HOME/serverloc, and ReadServerLoc() should read that info back", t, func() {
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
		cv.So(string(sl), cv.ShouldEqual, `export GOQ_JSERV_IP=10.0.0.6
export GOQ_JSERV_PORT=1776
export GOQ_SENDTIMEOUT_MSEC=1000
`)

		// fill cfg with some test garbage.
		orig := *cfg
		cfg.SendTimeoutMsec = -1
		cfg.JservIP = "0.0.0.0"
		cfg.JservPort = 9999
		cv.So(cfg.SendTimeoutMsec, cv.ShouldNotEqual, orig.SendTimeoutMsec)
		cv.So(cfg.JservIP, cv.ShouldNotEqual, orig.JservIP)
		cv.So(cfg.JservPort, cv.ShouldNotEqual, orig.JservPort)

		ReadServerLoc(cfg)
		fmt.Printf("\n    After reading our save serverloc info from file, the restored info should obliterate the test garbage we filled in.\n")
		cv.So(cfg.SendTimeoutMsec, cv.ShouldEqual, orig.SendTimeoutMsec)
		cv.So(cfg.JservIP, cv.ShouldEqual, orig.JservIP)
		cv.So(cfg.JservPort, cv.ShouldEqual, orig.JservPort)

	})
}
