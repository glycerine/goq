package main

import (
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

// cfgenv.go related test
//

func TestStripNanoAddr(t *testing.T) {

	cv.Convey("StripNanomsgAddressPrefix should omit the 'tcp://' stuff", t, func() {
		strip, err := StripNanomsgAddressPrefix("tcp://a.b.c.d:9090")
		cv.So(strip, cv.ShouldEqual, "a.b.c.d:9090")
		cv.So(err, cv.ShouldEqual, nil)
	})
}
