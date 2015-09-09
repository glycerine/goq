package main

import (
	"strings"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestWebGoesUpAndDown(t *testing.T) {

	// *** universal test cfg setup
	skipbye := false
	cfg := NewTestConfig() // bumps cfg.TestportBump so cfg.GetWebPort() is different in test.
	defer cfg.ByeTestConfig(&skipbye)
	// *** end universal test setup

	s := NewWebServer()
	cv.Convey("NewWebServer() should bring up a debug web-server", t, func() {
		cv.So(PortIsBound(s.Addr), cv.ShouldEqual, true)
		by, err := FetchUrl("http://" + s.Addr + "/debug/pprof")
		cv.So(err, cv.ShouldEqual, nil)
		//fmt.Printf("by:'%s'\n", string(by))
		cv.So(strings.HasPrefix(string(by), `<html>
<head>
<title>/debug/pprof/</title>
</head>`), cv.ShouldEqual, true)

	})

	cv.Convey("WebServer::Stop() should bring down the debug web-server", t, func() {
		s.Stop()
		cv.So(PortIsBound(s.Addr), cv.ShouldEqual, false)
	})
}
