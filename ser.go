package main

// ser.go : serialization

import (
	"bytes"
	"fmt"
	"io"
	"time"

	schema "github.com/glycerine/goq/schema"
	"github.com/glycerine/go-capnproto"
)

func (js *JobServ) ServerToCapnp() (bytes.Buffer, *capn.Segment) {
	seg := capn.NewBuffer(nil)
	z := schema.NewRootZ(seg)

	zjs := schema.NewZgoqserver(seg)
	z.SetGoqserver(zjs)

	// nextjobid
	zjs.SetNextjobid(js.NextJobId)

	var i int
	var j *Job

	// runq
	if len(js.RunQ) > 0 {
		//fmt.Printf("len of Runq: %d, %#v\n", len(js.RunQ), js.RunQ)
		runq := schema.NewZjobList(seg, len(js.RunQ))
		plistRunq := capn.PointerList(runq)
		i = 0
		for _, j = range js.RunQ {
			zjob := JobToCapnpSegment(j, seg)
			plistRunq.Set(i, capn.Object(zjob))
			i++
		}
		zjs.SetRunq(runq)
	}

	// waitingjobs
	if len(js.WaitingJobs) > 0 {
		//fmt.Printf("len of WaitingJobs: %d, %#v\n", len(js.WaitingJobs), js.WaitingJobs)
		waitingjobs := schema.NewZjobList(seg, len(js.WaitingJobs))
		plistWaitingjobs := capn.PointerList(waitingjobs)
		i = 0
		for _, j = range js.WaitingJobs {
			zjob := JobToCapnpSegment(j, seg)
			plistWaitingjobs.Set(i, capn.Object(zjob))
			i++
		}
		zjs.SetWaitingjobs(waitingjobs)
	}
	// counters
	zjs.SetFinishedjobscount(js.FinishedJobsCount)
	zjs.SetBadsgtcount(js.BadSgtCount)
	zjs.SetCancelledjobcount(js.CancelledJobCount)
	zjs.SetBadnoncecount(js.BadNonceCount)

	// FinishedRing -> finishedjobs
	if len(js.FinishedRing) > 0 {
		finishedjobs := schema.NewZjobList(seg, len(js.FinishedRing))
		plistFinishedjobs := capn.PointerList(finishedjobs)
		i = 0
		for _, j = range js.FinishedRing {
			zjob := JobToCapnpSegment(j, seg)
			plistFinishedjobs.Set(i, capn.Object(zjob))
			i++
		}
		zjs.SetFinishedjobs(finishedjobs)
	}

	buf := bytes.Buffer{}
	seg.WriteTo(&buf)

	return buf, seg
}

func (js *JobServ) SetStateFromCapnp(r io.Reader, fn string) {
	capMsg, err := capn.ReadFromStream(r, nil)
	if err != nil {
		panic(fmt.Errorf("capnp problem reading from file '%s': %s", fn, err))
	}

	z := schema.ReadRootZ(capMsg)
	d3 := z.Which()
	if d3 != schema.Z_GOQSERVER {
		panic(fmt.Sprintf("expected schema.Z_GOQSERVER, got %d", d3))
	}

	zjs := z.Goqserver()
	js.NextJobId = zjs.Nextjobid()

	runqlist := zjs.Runq().ToArray()
	for _, zjob := range runqlist {
		j := CapnpZjobToJob(zjob)

		// don't kill jobs prematurely just because we the server
		// just started back up!
		j.Unansweredping = 0
		j.Lastpingtm = time.Now().UnixNano()

		js.RunQ[j.Id] = j
		js.KnownJobHash[j.Id] = j
		js.RegisterWho(j)
	}

	waitlist := zjs.Waitingjobs().ToArray()
	for _, zjob := range waitlist {
		j := CapnpZjobToJob(zjob)
		js.WaitingJobs = append(js.WaitingJobs, j)
		js.KnownJobHash[j.Id] = j
		// getting too many open files errors, try doing
		// this on demand instead of all at once: comment it out.
		// js.RegisterWho(j)
	}

	js.FinishedJobsCount = zjs.Finishedjobscount()
	js.BadSgtCount = zjs.Badsgtcount()
	js.CancelledJobCount = zjs.Cancelledjobcount()
	js.BadNonceCount = zjs.Badnoncecount()

	finishedlist := zjs.Finishedjobs().ToArray()
	for _, zjob := range finishedlist {
		j := CapnpZjobToJob(zjob)
		js.FinishedRing = append(js.FinishedRing, j)
	}

}

func CapnpZjobToJob(zj schema.Zjob) *Job {

	return &Job{
		Id:       zj.Id(),
		Msg:      zj.Msg(),
		Aboutjid: zj.Aboutjid(),

		Cmd:  zj.Cmd(),
		Args: zj.Args().ToArray(),
		Out:  zj.Out().ToArray(),
		Env:  zj.Env().ToArray(),

		// err and failed
		Err:      zj.Err(),
		HadError: zj.Haderror(),

		Host: zj.Host(),
		Stm:  zj.Stm(),
		Etm:  zj.Etm(),

		Elapsec: zj.Elapsec(),
		Status:  zj.Status(),
		Subtime: zj.Subtime(),

		Pid: zj.Pid(),
		Dir: zj.Dir(),

		Submitaddr: zj.Submitaddr(),
		Serveraddr: zj.Serveraddr(),
		Workeraddr: zj.Workeraddr(),
		Finishaddr: zj.Finishaddr().ToArray(),

		Signature: zj.Signature(),
		IsLocal:   zj.Islocal(),
		Cancelled: zj.Cancelled(),

		ArrayId:        zj.Arrayid(),
		GroupId:        zj.Groupid(),
		Delegatetm:     zj.Delegatetm(),
		Lastpingtm:     zj.Lastpingtm(),
		Unansweredping: zj.Unansweredping(),
		Sendernonce:    zj.Sendernonce(),
		Sendtime:       zj.Sendtime(),
		MaxShow:        zj.Maxshow(),
		CmdOpts:        zj.Cmdopts(),
	}
}

func CapnpToJob(buf *bytes.Buffer) *Job {
	capMsg, err := capn.ReadFromStream(buf, nil)
	if err != nil {
		panic(err)
	}

	z := schema.ReadRootZ(capMsg)
	d3 := z.Which()
	if d3 != schema.Z_JOB {
		panic(fmt.Sprintf("expected schema.Z_JOB, got %d", d3))
	}

	zj := z.Job()
	job := CapnpZjobToJob(zj)

	return job
}

// once you've already allocated a segment
func JobToCapnpSegment(j *Job, seg *capn.Segment) schema.Zjob {

	zjob := schema.NewZjob(seg)

	zjob.SetId(j.Id)

	zjob.SetAboutjid(j.Aboutjid)
	zjob.SetMsg(j.Msg)

	zjob.SetCmd(j.Cmd)

	if tl := StringSliceToCapnp(j.Out, seg); tl != nil {
		zjob.SetOut(*tl)
	}

	if tl := StringSliceToCapnp(j.Args, seg); tl != nil {
		zjob.SetArgs(*tl)
	}

	if tl := StringSliceToCapnp(j.Env, seg); tl != nil {
		zjob.SetEnv(*tl)
	}

	// err and failed
	zjob.SetErr(j.Err)
	zjob.SetHaderror(j.HadError)

	zjob.SetHost(j.Host)

	zjob.SetStm(int64(j.Stm))
	zjob.SetEtm(int64(j.Etm))
	zjob.SetElapsec(j.Elapsec)

	zjob.SetStatus(j.Status)
	zjob.SetSubtime(int64(j.Subtime))
	zjob.SetPid(j.Pid)

	zjob.SetDir(j.Dir)

	zjob.SetSubmitaddr(j.Submitaddr)
	zjob.SetServeraddr(j.Serveraddr)
	zjob.SetWorkeraddr(j.Workeraddr)

	if tl := StringSliceToCapnp(j.Finishaddr, seg); tl != nil {
		zjob.SetFinishaddr(*tl)
	}

	zjob.SetSignature(j.Signature)
	zjob.SetIslocal(j.IsLocal)
	zjob.SetCancelled(j.Cancelled)

	zjob.SetArrayid(j.ArrayId)
	zjob.SetGroupid(j.GroupId)
	zjob.SetDelegatetm(j.Delegatetm)
	zjob.SetLastpingtm(j.Lastpingtm)
	zjob.SetUnansweredping(j.Unansweredping)
	zjob.SetSendernonce(j.Sendernonce)
	zjob.SetSendtime(j.Sendtime)
	zjob.SetMaxshow(j.MaxShow)
	zjob.SetCmdopts(j.CmdOpts)

	return zjob
}

func JobToCapnp(j *Job) (bytes.Buffer, *capn.Segment) {
	seg := capn.NewBuffer(nil)
	z := schema.NewRootZ(seg)

	zjob := JobToCapnpSegment(j, seg)
	z.SetJob(zjob)

	buf := bytes.Buffer{}
	seg.WriteTo(&buf)

	return buf, seg
}

func StringSliceToCapnp(a []string, seg *capn.Segment) *capn.TextList {
	if a == nil {
		panic("string slice can't be nil")
	}
	// looks like we need a size zero textlist for complete initialization,
	// so do this even if len(a) == 0
	tl := seg.NewTextList(len(a))
	for i := range a {
		tl.Set(i, a[i])
	}
	return &tl
}
