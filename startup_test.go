package main

import (
	"os"
	"path/filepath"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestStartupInHomeDir(t *testing.T) {

	cv.Convey("On 'goq serve' startup, the process should Chdir to GOQ_HOME", t, func() {

		var jobserv *JobServ
		var err error
		var jobservPid int
		remote := false

		// *** universal test cfg setup
		skipbye := false

		// this will move us to a new tempdir
		cfg := NewTestConfig()

		// now move away so we can check that there is a Chdir
		cv.So(cfg.tempdir, cv.ShouldNotEqual, cfg.origdir)

		err = os.Chdir("..")
		if err != nil {
			panic(err)
		}
		pwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		cv.So(pwd, cv.ShouldNotEqual, cfg.Home)

		defer cfg.ByeTestConfig(&skipbye)
		// *** end universal test setup

		cfg.DebugMode = true // reply to badsig packets

		// local only, because we use Getwd() to see what dir we are in.
		jobserv, err = NewJobServ(cfg)
		if err != nil {
			panic(err)
		}
		defer CleanupServer(cfg, jobservPid, jobserv, remote, &skipbye)
		defer CleanupOutdir(cfg)

		pwd, err = os.Getwd()
		if err != nil {
			panic(err)
		}
		epwd, _ := filepath.EvalSymlinks(pwd)
		ecfg, _ := filepath.EvalSymlinks(cfg.Home)

		cv.So(epwd, cv.ShouldEqual, ecfg)
	})
}
