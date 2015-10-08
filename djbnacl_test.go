package main

import (
	"fmt"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	nacl "golang.org/x/crypto/nacl/secretbox"
)

func TestDjbNaclSealOpen(t *testing.T) {
	var key [32]byte

	start := time.Now()
	orig := []byte("12345678901234567890123456789012")
	hashed := HashAlotSha256(orig)
	copy(key[:], hashed[:])
	fmt.Printf("that took: %v\n", time.Since(start)) // 80 usec

	cv.Convey("So that we can use the encyprtion of DJB's NaCl (salt) suite, we wrap with Seal/Open from crypto/nacl/secretbox.", t, func() {

		message := []byte("I'm a little teapot")

		var nonce, nonce2 [LEN_NONCE_BYTES]byte
		generateNonce24(&nonce)
		generateNonce24(&nonce2)

		//		fmt.Printf("nonce = '%#v'\n", nonce)
		//		fmt.Printf("nonce2 = '%#v'\n", nonce2)

		cv.Convey("nonce should not be all zeros: the nonce should have been intialized by generateNonce24()", func() {
			cv.So(nonce[:], cv.ShouldNotResemble, make([]byte, LEN_NONCE_BYTES))
		})

		cv.Convey("two different nonces should be different", func() {
			cv.So(nonce[:], cv.ShouldNotResemble, nonce2[:])
		})

		box := NaClEncryptWithRandomNoncePrepended(message, &key)
		opened, ok := NaclDecryptWithNoncePrepended(box, &key)

		cv.Convey("When a NaCl box is Sealed and then re-Opened, we should be able to open the box without error", func() {
			cv.So(ok, cv.ShouldBeTrue)
		})

		cv.Convey("  ... and should return the same plaintext", func() {
			cv.So(opened, cv.ShouldResemble, message)
		})

		cv.Convey("Integrity checking should catch any single bit modification of the cyphertext", func() {
			for i := range box {
				box[i] ^= 0x10
				_, ok := nacl.Open(nil, box, &nonce, &key)
				if ok {
					t.Fatalf("opened box with byte %d corrupted", i)
				}
				cv.So(ok, cv.ShouldBeFalse)

				box[i] ^= 0x10
			}
		})

	})
}
