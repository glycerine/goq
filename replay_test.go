package main

import (
	"fmt"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestReplayAttacksShouldNotSucceed(t *testing.T) {

	cv.Convey("Recording and replaying (replay attack) the same submit of a job should be impossible", t, func() {
		cv.Convey("so the client must include in each message a timestamp (in the Job.Sendtime field) and random client nonce (in Job.Sendernonce). If the message is too old (beyond 10 seconds), or the nonce is repeated, the server should reject it.", func() {
			cv.Convey("this avoids having to have an extra-roundtrip handshake before each submit to get a unique server-nonce", func() {
				cv.Convey("it also bounds the amount of memory the server must devote to deduping the Sendernonce; since only the last 10 seconds to be indexed", func() {

					var err error
					remote := false

					// *** universal test cfg setup
					skipbye := false
					cfg := NewTestConfig()
					//cfg.SendTimeoutMsec = 5000
					defer cfg.ByeTestConfig(&skipbye)
					// *** end universal test setup

					cfg.DebugMode = true // reply to badsig packets

					jobserv, jobservPid := HelperNewJobServ(cfg, remote)
					defer CleanupServer(cfg, jobservPid, jobserv, remote, nil)
					defer CleanupOutdir(cfg)

					j := NewJob()
					j.Cmd = "bin/sleep1.sh"

					sub, err := NewSubmitter(GenAddress(), cfg, false)
					if err != nil {
						panic(err)
					}

					_, err = sub.SubmitJobGetReply(j)
					if err != nil {
						panic(err)
					}

					// now try to replay this exact same message
					_, err = sendZjobWithoutStamping(sub.ServerPushSock, j, &sub.Cfg)
					if err != nil {
						panic(err)
					}

					// but with a new stamp, it should succed and not add another to badNonceCount
					j.Cmd = "bin/sleep2.sh"
					_, err = sub.SubmitJobGetReply(j)
					if err != nil {
						panic(err)
					}

					// check the badNonceCount
					serverSnap, err := SubmitGetServerSnapshot(cfg)
					if err != nil {
						panic(err)
					}
					snapmap := EnvToMap(serverSnap)
					fmt.Printf("serverSnap = %#v\n", serverSnap)

					cv.So(len(snapmap), cv.ShouldBeGreaterThan, 8)
					cv.So(snapmap["badNonceCount"], cv.ShouldEqual, "1") // not 0, not 2
					cv.So(snapmap["waitingJobs"], cv.ShouldEqual, "2")   // not 0, not 2

				})
			})
		})
	})
}

type TestTimeSource struct {
	MyNow Ntm
}

func (rts *TestTimeSource) Now() Ntm {
	return rts.MyNow
}

func NewTestTimeSource() *TestTimeSource {
	return &TestTimeSource{MyNow: 1}
}

func TestNonceRegistryTimesout(t *testing.T) {
	cv.Convey("a NonceRegistry should only keep values that are newer than its timeout", t, func() {
		tsrc := NewTestTimeSource()
		tsrc.MyNow = Ntm(1)
		reg := NewNonceRegistry(tsrc)

		j := NewJob()
		j.Sendtime = int64(1)
		tsrc.MyNow = Ntm(1 + reg.InvalidAfterDur - 1)
		b := reg.AddedOkay(j)
		fmt.Printf("AddedOkay(j) returned b = %v\n", b)
		cv.So(b, cv.ShouldEqual, true)

		badjob := NewJob()
		badjob.Sendtime = int64(1)
		tsrc.MyNow = Ntm(1 + reg.InvalidAfterDur)

		fmt.Printf("\n GCReg() should clean out j, now that tsrc.MyNow has advanced to where it is stale.\n")
		cv.So(len(reg.NonceHash), cv.ShouldEqual, 1)
		cv.So(reg.TimeTree.Len(), cv.ShouldEqual, 1)
		reg.GCReg()
		cv.So(len(reg.NonceHash), cv.ShouldEqual, 0)
		cv.So(reg.TimeTree.Len(), cv.ShouldEqual, 0)

		b2 := reg.AddedOkay(badjob)
		cv.So(b2, cv.ShouldEqual, false)

		fmt.Printf("\n GCReg shouldn't keep going into even younger times.\n")
		tsrc.MyNow = Ntm(5)
		reg.InvalidAfterDur = 10

		j1 := NewJob()
		j1.Sendtime = 1
		cv.So(reg.AddedOkay(j1), cv.ShouldEqual, true)

		j2 := NewJob()
		j2.Sendtime = 2
		cv.So(reg.AddedOkay(j2), cv.ShouldEqual, true)

		j3 := NewJob()
		j3.Sendtime = 3
		cv.So(reg.AddedOkay(j3), cv.ShouldEqual, true)

		j4 := NewJob()
		j4.Sendtime = 4
		cv.So(reg.AddedOkay(j4), cv.ShouldEqual, true)

		fmt.Printf("\n   GCReg should only do the minimal number of comparisons needed\n")
		fmt.Printf("      to determine which stale jobs to evict (user the rbtree to maximal effect to avoid linear scan)\n")
		tsrc.MyNow = Ntm(4)
		reg.InvalidAfterDur = 2
		cv.So(reg.GCReg(), cv.ShouldEqual, 3) // not 4! should stop at 2, skipping 1.
	})
}
