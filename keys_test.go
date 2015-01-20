package main

import (
	"fmt"
	"os"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestGeneratingNewKeys(t *testing.T) {

	// setup a testing context where the
	// usual calls on a clean directory can succeed.
	// Despite the fact that other servers have
	// already initialized the .goq/ directory.
	origdir, tmpdir := MakeAndMoveToTempDir()

	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	cfg := DefaultCfg()
	cfg.Home = pwd

	var key *CypherKey

	cv.Convey("Upon request, we should be able check for keys, create new keys, and load existing keys to/from our GOQ_HOME/.goq directory", t, func() {
		cv.Convey("Check for existing keys should flounder in a fresh directory", func() {
			res := KeyExists(cfg)
			cv.So(res, cv.ShouldEqual, false)
		})
		cv.Convey("Create new keys should succeed", func() {
			key, err = NewKey(cfg)
			cv.So(err, cv.ShouldEqual, nil)
			cv.So(key, cv.ShouldNotEqual, nil)
		})
		cv.Convey("Load keys, now that they are created, should succeed", func() {
			key, err = LoadKey(cfg)
			cv.So(err, cv.ShouldEqual, nil)

			cv.So(key, cv.ShouldNotEqual, nil)
			cv.So(key.IsValid(), cv.ShouldEqual, true)
		})
		cv.Convey("Check for existing keys should succeed once we've already made them.", func() {
			res := KeyExists(cfg)
			cv.So(res, cv.ShouldEqual, true)
		})
		cv.Convey("Delete key should succeed", func() {
			err = key.DeleteKey()
			cv.So(err, cv.ShouldEqual, nil)
			cv.So(KeyExists(cfg), cv.ShouldEqual, false)
		})
		cv.Convey("Create after Delete should succeed", func() {
			key, err = NewKey(cfg)
			cv.So(err, cv.ShouldEqual, nil)
			cv.So(key, cv.ShouldNotEqual, nil)
		})

		cv.Convey("Cyperttext before and after a change of keys should differ", func() {
			err = key.DeleteKey()
			cv.So(err, cv.ShouldEqual, nil)

			key, err = NewKey(cfg)
			cv.So(err, cv.ShouldEqual, nil)
			cv.So(key, cv.ShouldNotEqual, nil)

			plain := "hello world"
			cy1 := key.Encrypt([]byte(plain))
			p1 := key.Decrypt(cy1)

			err = key.DeleteKey()
			cv.So(err, cv.ShouldEqual, nil)

			key, err = NewKey(cfg)
			cv.So(err, cv.ShouldEqual, nil)
			cv.So(key, cv.ShouldNotEqual, nil)

			cy2 := key.Encrypt([]byte(plain))
			p2 := key.Decrypt(cy2)

			cv.So(cy1, cv.ShouldNotEqual, cy2)
			cv.So(string(p1), cv.ShouldEqual, plain)
			cv.So(string(p2), cv.ShouldEqual, plain)

			fmt.Printf("\n   plain: '%s'\n   cyper1: '%s'\n   cyper2: '%s'\n   decrypted cy1: '%s'\n   decrypted cy2: '%s'\n", plain, cy1, cy2, p1, p2)
		})

	})

	// cleanup
	TempDirCleanup(origdir, tmpdir)
}
