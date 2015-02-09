package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	nn "github.com/glycerine/mangos/compat"
)

//
// simple test of mangos connect to unused address and send timeout
//

func TestSendToUnboundAddressShouldTimeout005(t *testing.T) {

	cv.Convey("remotely, over nanomsg, a send to a non-existant address should timeout and fail", t, func() {

		unused_addr := GenAddress()

		push1, err := nn.NewSocket(nn.AF_SP, nn.PUSH)
		if err != nil {
			panic(err)
		}
		SendTimeoutMsec := 3000
		err = push1.SetSendTimeout(time.Duration(SendTimeoutMsec) * time.Millisecond)
		if err != nil {
			panic(err)
		}

		_, err = push1.Connect(unused_addr)
		if err != nil {
			fmt.Printf("\ncould not bind addr '%s': %v\n", unused_addr, err)
			// correct
			cv.So(err, cv.ShouldNotEqual, nil)
			return
		}
		// wrong? but err is nil.
		//cv.So(err, cv.ShouldNotEqual, nil)
		fmt.Printf("\n[pid %d] push socket made at '%s'.\n", os.Getpid(), unused_addr)

		cy := []byte("hello")
		_, err = push1.Send(cy, 0)

		// really should not be able to Send on a not-connected socket.
		fmt.Printf("err was: '%s'\n", err) // nil under mangos. 'resource temporarily unavailable' when using C lib nanomsg.
		cv.So(err, cv.ShouldNotEqual, nil)
	})
}
