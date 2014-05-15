package main

import (
	"os/exec"
	"strings"
	"testing"

	cv "github.com/smartystreets/goconvey/convey"
)

//func TestDemo(t *testing.T) {
//	t.Logf("use the -test.v flag to see this output")
//}

// basic functionality:

// starting server should bind the supplied endpoint
func TestServerBinds(t *testing.T) {
	cfg := GetEnvConfig(RandId)
	ServerBindHelper(t, cfg.JservAddr, cfg.JservAddr, cfg)
}

// and the netstat validation should implode if we
// give the wrong endpoint:

func TestBadEndpointMeansServerEndpointTestShouldImplode(t *testing.T) {
	cfg := GetEnvConfig(RandId)
	cv.Convey("bad endpoints should be detected and rejected", t, func() {
		cv.ShouldPanic(func() { panic("test the goconvey ShouldPanic function") })
		cv.ShouldPanic(func() { ServerBindHelper(t, "tcp://127.0.0.1:1776", "tcp://127.0.0.1:1779", cfg) })
		cv.ShouldPanic(func() { ServerBindHelper(t, "tcp://127.0.0.1:1777", "tcp://127.0.0.1:1778", cfg) })
		cv.ShouldNotPanic(func() { ServerBindHelper(t, "tcp://127.0.0.1:1779", "tcp://127.0.0.1:1779", cfg) })
	})
}

// make addr separate from cfg.JservAddr, so we
// can validate that the test detects a problem
// when they are different.
func ServerBindHelper(t *testing.T, addr_use string, addr_expect string, cfg *Config) {
	/*	nnzbus, err := nn.NewSocket(nn.AF_SP, nn.PAIR)
		if err != nil {
			t.Fatal(err)
		}
	*/
	serv, err := NewJobServ(addr_use, cfg)
	if err != nil {
		panic(err)
	}
	defer serv.Nnsock.Close()
	defer CleanupOutdir(cfg)

	found := PortIsListenedOn(t, addr_expect)

	if !found {
		//t.Logf("gozbus server was not listening on %v as expected", addr_expect)
		panic("no gozbus server at expected endpoint")
	}
}

func PortIsListenedOn(t *testing.T, addr_expect string) bool {
	// Equivalent to: $(netstat -nuptl| grep <ip:port> | grep LISTEN).
	// Input addr_expect: is of the form "tcp://127.0.0.1:1777"
	// and we discard everyting before the // to get ip:port.

	out, err := exec.Command("netstat", "-nuptl").Output()
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(out), "\n")
	needle := strings.SplitAfter(addr_expect, "//")[1]
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

// client should be able to send to live server

// client should be receive from live server

// server should be able to send to client

// server should be able to receive from client

// go convey try out:

func TestClientToServer(t *testing.T) {

	// Only pass t into top-level Convey calls
	cv.Convey("client should be able to talk to server", t, func() {

	})
}
