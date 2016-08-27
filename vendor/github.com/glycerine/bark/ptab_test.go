package bark

import (
	"os"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestProcessTable(t *testing.T) {

	cv.Convey("Sanity check ProcessTable(): ProcessTable should give us a map of length > 2", t, func() {
		cv.Convey("and should contain our and our parents pid", func() {
			ptab := *ProcessTable()
			me := os.Getpid()
			par := os.Getppid()

			_, meFound := ptab[me]
			_, parFound := ptab[par]

			cv.So(len(ptab), cv.ShouldBeGreaterThan, 2)
			cv.So(meFound, cv.ShouldEqual, true)
			cv.So(parFound, cv.ShouldEqual, true)

			Q("Sysname = '%s'\n", Sysname)
			Q("ptab is: %#v\n", ptab)
		})
	})
}
