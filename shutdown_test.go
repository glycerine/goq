package main

import (
	"testing"

	cv "github.com/smartystreets/goconvey/convey"
)

func TestLocalNanomsgBasedShutdown(t *testing.T) {

	cv.Convey("shutdown after nanomsg communication should be clean", t, func() {
		cv.Convey("even if we use mangos", func() {

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			jobserv, err := NewJobServ(cfg)
			if err != nil {
				panic(err)
			}
			remote := false
			jobservPid := 0
			defer CleanupServer(cfg, jobservPid, jobserv, remote, &skipbye)
			defer CleanupOutdir(cfg)

			sub, err := NewSubmitter(GenAddress(), cfg, false)
			if err != nil {
				panic(err)
			}
			sub.SubmitShutdownJob()

			//cv.So(jobout.Out[1], cv.ShouldEqual, "I'm done with my work")

		})
	})
}
