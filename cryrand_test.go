package main

import (
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestCryptoRandReturnsUniqueInt64(t *testing.T) {

	cv.Convey("calls to our CryptoRandInt64() function should not return the same integer in sequence (with 2^-64 probability of failure)", t, func() {
		a := CryptoRandInt64()
		b := CryptoRandInt64()
		cv.So(a, cv.ShouldNotEqual, b)
	})
}
