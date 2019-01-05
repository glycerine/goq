package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/glycerine/rbtree"
)

// prevent replay attacks by detecting jobs with
// timestamps that are too old or Nonce's that are not unique

type Nonce int64

type NonceRegistry struct {
	TSrc            TimeSource
	TimeTree        *rbtree.Tree
	NonceHash       map[Nonce]Ntm
	InvalidAfterDur Ntm // in nanoseconds
}

type RealTimeSource struct{}

func NewRealTimeSource() *RealTimeSource {
	return &RealTimeSource{}
}

func (rts *RealTimeSource) Now() Ntm {
	return Ntm(time.Now().UnixNano())
}

type TimeSource interface {
	Now() Ntm
}

func NewNonceRegistry(tsrc TimeSource) *NonceRegistry {

	return &NonceRegistry{
		TSrc: tsrc,
		TimeTree: rbtree.NewTree(func(a, b rbtree.Item) int {
			return int(a.(*Job).Sendtime - b.(*Job).Sendtime)
		}),
		NonceHash:       make(map[Nonce]Ntm),
		InvalidAfterDur: Ntm(3600e9), // 1 hour (in nanoseconds)
	}

}

func (n *NonceRegistry) IsBadStamp(j *Job) bool {
	n.GCReg()
	if n.tooOld(j) {
		if ShowSig {
			AlwaysPrintf("detected job tooOld(): %s", j)
			AlwaysPrintf("debug: NonceRegistry = %s", n)
		}
		return true
	}
	if _, ok := n.NonceHash[Nonce(j.Sendernonce)]; ok {
		if ShowSig {
			AlwaysPrintf("\n detected replay of duplicate nonce: %x from job: %s\n", j.Sendernonce, j)
			AlwaysPrintf("debug: NonceRegistry = %s\n", n)
		}
		return true
	}
	return false
}

func (n *NonceRegistry) AddedOkay(j *Job) bool {
	VPrintf("debug: In AddedOkay(j.Sendernonce=%x): NonceRegistry = %s\n", j.Sendernonce, n)

	if j == nil {
		panic("j cannot be nil")
	}
	if n.IsBadStamp(j) {
		//vv("NonceRegister.AddedOkay sees BadStamp")
		return false
	}
	n.TimeTree.Insert(j)
	n.NonceHash[Nonce(j.Sendernonce)] = Ntm(j.Sendtime)
	return true
}

func (n *NonceRegistry) String() string {
	s := n.TimeTreeAsString()
	s += "\nNonceHash:\n"
	for k, v := range n.NonceHash {
		s += fmt.Sprintf("   Nonce: %x, Ntm: %s (%d)\n", k, time.Unix(int64(v/1e9), int64(v%1e9)), v)
	}
	return s
}

func (n *NonceRegistry) TimeTreeAsString() string {
	r := "\n"
	for it := n.TimeTree.Min(); !it.Limit(); it = it.Next() {
		j := it.Item().(*Job)
		r += fmt.Sprintf(" TimeTree: Ntm:%s (%d)  Job.Nonce: %x\n", time.Unix(j.Sendtime/1e9, j.Sendtime%1e9), j.Sendtime, j.Sendernonce)
	}
	return r
}

func (n *NonceRegistry) tooOld(j *Job) bool {
	//vv("tooOld top: j.SendTime='%v'", j.Sendtime)
	now := n.TSrc.Now()

	if j.Sendtime == 0 {
		//vv("j.Sendtime was 0! tooOld returning true")
		return true
	}
	diff := now - Ntm(j.Sendtime)
	if diff >= n.InvalidAfterDur {
		//vv("now minus j.Sendtime == %v > invalidAfter = '%v'; tooOld returning true", diff, n.InvalidAfterDur)
		return true
	}
	//vv("tooOld returning false")
	return false
}

func (n *NonceRegistry) TooNew(j *Job) (bool, Ntm) {
	now := n.TSrc.Now()
	fut := Ntm(j.Sendtime) - now
	if fut > n.InvalidAfterDur {
		// reject jobs from the future
		//vv("rejecting job from the future")
		return true, fut
	}
	return false, fut
}

// GCReg: garbage collect old entries
// returns the number of timestamp points that were scanned in the tree.
func (n *NonceRegistry) GCReg() int {
	scanned := 0
	it := n.TimeTree.Min()
	for !it.Limit() {
		j := it.Item().(*Job)
		scanned++
		if n.tooOld(j) {
			//fmt.Printf("CGReg detected stale job in registry, deleting: %s\n", j)
			nonce := Nonce(j.Sendernonce)

			// advance before deleting...
			it = it.Next()
			n.TimeTree.DeleteWithKey(j)

			// bound the size of our NonceHash here.
			// We are limited to just the young jobs' Sendernonce.
			delete(n.NonceHash, nonce)
		} else {
			// no need to go further into younger jobs. Avoid full linear scan.
			break
			//fmt.Printf("\nskipping scan of job with even younger time: %d\n", j.Sendtime)
			//it = it.Next()
		}
	}
	return scanned
}

// called from NewJob, can't call in SignJob() because that
// is used for verification too.
func StampJob(j *Job) {
	j.Sendtime = int64(time.Now().UnixNano())
	j.Sendernonce = int64(rand.Int())
}
