package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

//func TestDemo(t *testing.T) {
//	t.Logf("use the -test.v flag to see this output")
//}

// basic functionality:

// starting server should bind the supplied endpoint
func TestServerBinds(t *testing.T) {

	// *** universal test cfg setup
	skipbye := false
	cfg := NewTestConfig()
	defer cfg.ByeTestConfig(&skipbye)
	// *** end universal test setup

	ServerBindHelper(t, cfg.JservPort, cfg.JservPort, cfg)
}

// and the netstat validation should implode if we
// give the wrong endpoint:

func TestBadEndpointMeansServerEndpointTestShouldImplode(t *testing.T) {

	// *** universal test cfg setup
	skipbye := false
	cfg := NewTestConfig()
	defer cfg.ByeTestConfig(&skipbye)
	// *** end universal test setup

	cv.Convey("bad endpoints should be detected and rejected", t, func() {
		cv.ShouldPanic(func() { panic("test the goconvey ShouldPanic function") })
		cv.ShouldPanic(func() { ServerBindHelper(t, 2776, 2779, cfg) })
		cv.ShouldPanic(func() { ServerBindHelper(t, 2777, 2778, cfg) })
		cv.ShouldNotPanic(func() { ServerBindHelper(t, 2779, 2779, cfg) })
	})
}

// make addr separate from cfg.JservAddr, so we
// can validate that the test detects a problem
// when they are different.
func ServerBindHelper(t *testing.T, port_use int, port_expect int, cfg *Config) {

	cfg.JservPort = port_use
	serv, err := NewJobServ(cfg)
	if err != nil {
		panic(err)
	}
	defer serv.Nnsock.Close()
	defer CleanupOutdir(cfg)
	defer CleanupServer(cfg, -1, serv, false, nil)

	addr_expect := fmt.Sprintf(":%d", port_expect)
	found := PortIsListenedOn(t, addr_expect)

	if !found {
		panic("no server at expected endpoint")
	}
}

func PortIsListenedOn(t *testing.T, addr_expect string) bool {
	// Equivalent to: $(netstat -nuptl| grep <ip:port> | grep LISTEN).
	// Input addr_expect: is of the form "tcp://127.0.0.1:1777"
	// and we discard everyting before the // to get ip:port.

	netstatcmd, netstatargs := netstat_commandline()
	out, err := exec.Command(netstatcmd, netstatargs...).Output()

	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(out), "\n")
	needle := addr_expect
	//e.g.	needle := "127.0.0.1:1776"

	//t.Logf("using netstat -nuptl to look for %v LISTEN line.\n", needle)

	found := false
	for _, haystack := range lines {
		if strings.Contains(haystack, needle) &&
			strings.Contains(haystack, "LISTEN") {
			found = true
			//t.Logf("'netstat -nuptl | grep %v | grep LISTEN' found: %v", needle, haystack)
			break
		}
	}
	return found
}
