package goq

// AUTO GENERATED - DO NOT EDIT

import (
	C "github.com/glycerine/go-capnproto"
	"unsafe"
)

type JobMsg uint16

const (
	JOBMSG_INITIALSUBMIT     JobMsg = 0
	JOBMSG_ACKSUBMIT         JobMsg = 1
	JOBMSG_REQUESTFORWORK    JobMsg = 2
	JOBMSG_DELEGATETOWORKER  JobMsg = 3
	JOBMSG_SHUTDOWNWORKER    JobMsg = 4
	JOBMSG_ACKSHUTDOWNWORKER JobMsg = 5
	JOBMSG_FINISHEDWORK      JobMsg = 6
	JOBMSG_ACKFINISHED       JobMsg = 7
	JOBMSG_SHUTDOWNSERV      JobMsg = 8
	JOBMSG_ACKSHUTDOWNSERV   JobMsg = 9
	JOBMSG_CANCELWIP         JobMsg = 10
	JOBMSG_ACKCANCELWIP      JobMsg = 11
	JOBMSG_CANCELSUBMIT      JobMsg = 12
	JOBMSG_ACKCANCELSUBMIT   JobMsg = 13
	JOBMSG_TAKESNAPSHOT      JobMsg = 14
	JOBMSG_ACKTAKESNAPSHOT   JobMsg = 15
	JOBMSG_RESUBMITNOACK     JobMsg = 16
	JOBMSG_REJECTBADSIG      JobMsg = 17
	JOBMSG_OBSERVEJOBFINISH  JobMsg = 18
	JOBMSG_JOBFINISHEDNOTICE JobMsg = 19
	JOBMSG_JOBNOTKNOWN       JobMsg = 20
	JOBMSG_IMMOLATEAWORKERS  JobMsg = 21
	JOBMSG_IMMOLATEACK       JobMsg = 22
	JOBMSG_PINGWORKER        JobMsg = 23
	JOBMSG_ACKPINGWORKER     JobMsg = 24
)

func (c JobMsg) String() string {
	switch c {
	case JOBMSG_INITIALSUBMIT:
		return "initialsubmit"
	case JOBMSG_ACKSUBMIT:
		return "acksubmit"
	case JOBMSG_REQUESTFORWORK:
		return "requestforwork"
	case JOBMSG_DELEGATETOWORKER:
		return "delegatetoworker"
	case JOBMSG_SHUTDOWNWORKER:
		return "shutdownworker"
	case JOBMSG_ACKSHUTDOWNWORKER:
		return "ackshutdownworker"
	case JOBMSG_FINISHEDWORK:
		return "finishedwork"
	case JOBMSG_ACKFINISHED:
		return "ackfinished"
	case JOBMSG_SHUTDOWNSERV:
		return "shutdownserv"
	case JOBMSG_ACKSHUTDOWNSERV:
		return "ackshutdownserv"
	case JOBMSG_CANCELWIP:
		return "cancelwip"
	case JOBMSG_ACKCANCELWIP:
		return "ackcancelwip"
	case JOBMSG_CANCELSUBMIT:
		return "cancelsubmit"
	case JOBMSG_ACKCANCELSUBMIT:
		return "ackcancelsubmit"
	case JOBMSG_TAKESNAPSHOT:
		return "takesnapshot"
	case JOBMSG_ACKTAKESNAPSHOT:
		return "acktakesnapshot"
	case JOBMSG_RESUBMITNOACK:
		return "resubmitnoack"
	case JOBMSG_REJECTBADSIG:
		return "rejectbadsig"
	case JOBMSG_OBSERVEJOBFINISH:
		return "observejobfinish"
	case JOBMSG_JOBFINISHEDNOTICE:
		return "jobfinishednotice"
	case JOBMSG_JOBNOTKNOWN:
		return "jobnotknown"
	case JOBMSG_IMMOLATEAWORKERS:
		return "immolateaworkers"
	case JOBMSG_IMMOLATEACK:
		return "immolateack"
	case JOBMSG_PINGWORKER:
		return "pingworker"
	case JOBMSG_ACKPINGWORKER:
		return "ackpingworker"
	default:
		return ""
	}
}

func JobMsgFromString(c string) JobMsg {
	switch c {
	case "initialsubmit":
		return JOBMSG_INITIALSUBMIT
	case "acksubmit":
		return JOBMSG_ACKSUBMIT
	case "requestforwork":
		return JOBMSG_REQUESTFORWORK
	case "delegatetoworker":
		return JOBMSG_DELEGATETOWORKER
	case "shutdownworker":
		return JOBMSG_SHUTDOWNWORKER
	case "ackshutdownworker":
		return JOBMSG_ACKSHUTDOWNWORKER
	case "finishedwork":
		return JOBMSG_FINISHEDWORK
	case "ackfinished":
		return JOBMSG_ACKFINISHED
	case "shutdownserv":
		return JOBMSG_SHUTDOWNSERV
	case "ackshutdownserv":
		return JOBMSG_ACKSHUTDOWNSERV
	case "cancelwip":
		return JOBMSG_CANCELWIP
	case "ackcancelwip":
		return JOBMSG_ACKCANCELWIP
	case "cancelsubmit":
		return JOBMSG_CANCELSUBMIT
	case "ackcancelsubmit":
		return JOBMSG_ACKCANCELSUBMIT
	case "takesnapshot":
		return JOBMSG_TAKESNAPSHOT
	case "acktakesnapshot":
		return JOBMSG_ACKTAKESNAPSHOT
	case "resubmitnoack":
		return JOBMSG_RESUBMITNOACK
	case "rejectbadsig":
		return JOBMSG_REJECTBADSIG
	case "observejobfinish":
		return JOBMSG_OBSERVEJOBFINISH
	case "jobfinishednotice":
		return JOBMSG_JOBFINISHEDNOTICE
	case "jobnotknown":
		return JOBMSG_JOBNOTKNOWN
	case "immolateaworkers":
		return JOBMSG_IMMOLATEAWORKERS
	case "immolateack":
		return JOBMSG_IMMOLATEACK
	case "pingworker":
		return JOBMSG_PINGWORKER
	case "ackpingworker":
		return JOBMSG_ACKPINGWORKER
	default:
		return 0
	}
}

type JobMsg_List C.PointerList

func NewJobMsgList(s *C.Segment, sz int) JobMsg_List { return JobMsg_List(s.NewUInt16List(sz)) }
func (s JobMsg_List) Len() int                       { return C.UInt16List(s).Len() }
func (s JobMsg_List) At(i int) JobMsg                { return JobMsg(C.UInt16List(s).At(i)) }
func (s JobMsg_List) ToArray() []JobMsg {
	return *(*[]JobMsg)(unsafe.Pointer(C.UInt16List(s).ToEnumArray()))
}

// capn.JSON_enabled == false so we stub MarshallJSON().
func (s JobMsg) MarshalJSON() (bs []byte, err error) { return }

type Zjob C.Struct

func NewZjob(s *C.Segment) Zjob           { return Zjob(s.NewStruct(136, 13)) }
func NewRootZjob(s *C.Segment) Zjob       { return Zjob(s.NewRootStruct(136, 13)) }
func AutoNewZjob(s *C.Segment) Zjob       { return Zjob(s.NewStructAR(136, 13)) }
func ReadRootZjob(s *C.Segment) Zjob      { return Zjob(s.Root(0).ToStruct()) }
func (s Zjob) Id() int64                  { return int64(C.Struct(s).Get64(0)) }
func (s Zjob) SetId(v int64)              { C.Struct(s).Set64(0, uint64(v)) }
func (s Zjob) Msg() JobMsg                { return JobMsg(C.Struct(s).Get16(8)) }
func (s Zjob) SetMsg(v JobMsg)            { C.Struct(s).Set16(8, uint16(v)) }
func (s Zjob) Aboutjid() int64            { return int64(C.Struct(s).Get64(16)) }
func (s Zjob) SetAboutjid(v int64)        { C.Struct(s).Set64(16, uint64(v)) }
func (s Zjob) Cmd() string                { return C.Struct(s).GetObject(0).ToText() }
func (s Zjob) SetCmd(v string)            { C.Struct(s).SetObject(0, s.Segment.NewText(v)) }
func (s Zjob) Args() C.TextList           { return C.TextList(C.Struct(s).GetObject(1)) }
func (s Zjob) SetArgs(v C.TextList)       { C.Struct(s).SetObject(1, C.Object(v)) }
func (s Zjob) Out() C.TextList            { return C.TextList(C.Struct(s).GetObject(2)) }
func (s Zjob) SetOut(v C.TextList)        { C.Struct(s).SetObject(2, C.Object(v)) }
func (s Zjob) Env() C.TextList            { return C.TextList(C.Struct(s).GetObject(3)) }
func (s Zjob) SetEnv(v C.TextList)        { C.Struct(s).SetObject(3, C.Object(v)) }
func (s Zjob) Host() string               { return C.Struct(s).GetObject(4).ToText() }
func (s Zjob) SetHost(v string)           { C.Struct(s).SetObject(4, s.Segment.NewText(v)) }
func (s Zjob) Stm() int64                 { return int64(C.Struct(s).Get64(24)) }
func (s Zjob) SetStm(v int64)             { C.Struct(s).Set64(24, uint64(v)) }
func (s Zjob) Etm() int64                 { return int64(C.Struct(s).Get64(32)) }
func (s Zjob) SetEtm(v int64)             { C.Struct(s).Set64(32, uint64(v)) }
func (s Zjob) Elapsec() int64             { return int64(C.Struct(s).Get64(40)) }
func (s Zjob) SetElapsec(v int64)         { C.Struct(s).Set64(40, uint64(v)) }
func (s Zjob) Status() string             { return C.Struct(s).GetObject(5).ToText() }
func (s Zjob) SetStatus(v string)         { C.Struct(s).SetObject(5, s.Segment.NewText(v)) }
func (s Zjob) Subtime() int64             { return int64(C.Struct(s).Get64(48)) }
func (s Zjob) SetSubtime(v int64)         { C.Struct(s).Set64(48, uint64(v)) }
func (s Zjob) Pid() int64                 { return int64(C.Struct(s).Get64(56)) }
func (s Zjob) SetPid(v int64)             { C.Struct(s).Set64(56, uint64(v)) }
func (s Zjob) Dir() string                { return C.Struct(s).GetObject(6).ToText() }
func (s Zjob) SetDir(v string)            { C.Struct(s).SetObject(6, s.Segment.NewText(v)) }
func (s Zjob) Submitaddr() string         { return C.Struct(s).GetObject(7).ToText() }
func (s Zjob) SetSubmitaddr(v string)     { C.Struct(s).SetObject(7, s.Segment.NewText(v)) }
func (s Zjob) Serveraddr() string         { return C.Struct(s).GetObject(8).ToText() }
func (s Zjob) SetServeraddr(v string)     { C.Struct(s).SetObject(8, s.Segment.NewText(v)) }
func (s Zjob) Workeraddr() string         { return C.Struct(s).GetObject(9).ToText() }
func (s Zjob) SetWorkeraddr(v string)     { C.Struct(s).SetObject(9, s.Segment.NewText(v)) }
func (s Zjob) Finishaddr() C.TextList     { return C.TextList(C.Struct(s).GetObject(10)) }
func (s Zjob) SetFinishaddr(v C.TextList) { C.Struct(s).SetObject(10, C.Object(v)) }
func (s Zjob) Signature() string          { return C.Struct(s).GetObject(11).ToText() }
func (s Zjob) SetSignature(v string)      { C.Struct(s).SetObject(11, s.Segment.NewText(v)) }
func (s Zjob) Islocal() bool              { return C.Struct(s).Get1(80) }
func (s Zjob) SetIslocal(v bool)          { C.Struct(s).Set1(80, v) }
func (s Zjob) Arrayid() int64             { return int64(C.Struct(s).Get64(64)) }
func (s Zjob) SetArrayid(v int64)         { C.Struct(s).Set64(64, uint64(v)) }
func (s Zjob) Groupid() int64             { return int64(C.Struct(s).Get64(72)) }
func (s Zjob) SetGroupid(v int64)         { C.Struct(s).Set64(72, uint64(v)) }
func (s Zjob) Cancelled() bool            { return C.Struct(s).Get1(81) }
func (s Zjob) SetCancelled(v bool)        { C.Struct(s).Set1(81, v) }
func (s Zjob) Delegatetm() int64          { return int64(C.Struct(s).Get64(80)) }
func (s Zjob) SetDelegatetm(v int64)      { C.Struct(s).Set64(80, uint64(v)) }
func (s Zjob) Lastpingtm() int64          { return int64(C.Struct(s).Get64(88)) }
func (s Zjob) SetLastpingtm(v int64)      { C.Struct(s).Set64(88, uint64(v)) }
func (s Zjob) Unansweredping() int64      { return int64(C.Struct(s).Get64(96)) }
func (s Zjob) SetUnansweredping(v int64)  { C.Struct(s).Set64(96, uint64(v)) }
func (s Zjob) Sendernonce() int64         { return int64(C.Struct(s).Get64(104)) }
func (s Zjob) SetSendernonce(v int64)     { C.Struct(s).Set64(104, uint64(v)) }
func (s Zjob) Sendtime() int64            { return int64(C.Struct(s).Get64(112)) }
func (s Zjob) SetSendtime(v int64)        { C.Struct(s).Set64(112, uint64(v)) }
func (s Zjob) Err() string                { return C.Struct(s).GetObject(12).ToText() }
func (s Zjob) SetErr(v string)            { C.Struct(s).SetObject(12, s.Segment.NewText(v)) }
func (s Zjob) Haderror() bool             { return C.Struct(s).Get1(82) }
func (s Zjob) SetHaderror(v bool)         { C.Struct(s).Set1(82, v) }
func (s Zjob) Maxshow() int64             { return int64(C.Struct(s).Get64(120)) }
func (s Zjob) SetMaxshow(v int64)         { C.Struct(s).Set64(120, uint64(v)) }
func (s Zjob) Cmdopts() uint64            { return C.Struct(s).Get64(128) }
func (s Zjob) SetCmdopts(v uint64)        { C.Struct(s).Set64(128, v) }

// capn.JSON_enabled == false so we stub MarshallJSON().
func (s Zjob) MarshalJSON() (bs []byte, err error) { return }

type Zjob_List C.PointerList

func NewZjobList(s *C.Segment, sz int) Zjob_List { return Zjob_List(s.NewCompositeList(136, 13, sz)) }
func (s Zjob_List) Len() int                     { return C.PointerList(s).Len() }
func (s Zjob_List) At(i int) Zjob                { return Zjob(C.PointerList(s).At(i).ToStruct()) }
func (s Zjob_List) ToArray() []Zjob              { return *(*[]Zjob)(unsafe.Pointer(C.PointerList(s).ToArray())) }
func (s Zjob_List) Set(i int, item Zjob)         { C.PointerList(s).Set(i, C.Object(item)) }

type Z C.Struct
type Z_Which uint16

const (
	Z_NOTHING   Z_Which = 0
	Z_JOB       Z_Which = 1
	Z_GOQSERVER Z_Which = 2
)

func NewZ(s *C.Segment) Z             { return Z(s.NewStruct(16, 1)) }
func NewRootZ(s *C.Segment) Z         { return Z(s.NewRootStruct(16, 1)) }
func AutoNewZ(s *C.Segment) Z         { return Z(s.NewStructAR(16, 1)) }
func ReadRootZ(s *C.Segment) Z        { return Z(s.Root(0).ToStruct()) }
func (s Z) Which() Z_Which            { return Z_Which(C.Struct(s).Get16(8)) }
func (s Z) Nothing() int64            { return int64(C.Struct(s).Get64(0)) }
func (s Z) SetNothing(v int64)        { C.Struct(s).Set16(8, 0); C.Struct(s).Set64(0, uint64(v)) }
func (s Z) Job() Zjob                 { return Zjob(C.Struct(s).GetObject(0).ToStruct()) }
func (s Z) SetJob(v Zjob)             { C.Struct(s).Set16(8, 1); C.Struct(s).SetObject(0, C.Object(v)) }
func (s Z) Goqserver() Zgoqserver     { return Zgoqserver(C.Struct(s).GetObject(0).ToStruct()) }
func (s Z) SetGoqserver(v Zgoqserver) { C.Struct(s).Set16(8, 2); C.Struct(s).SetObject(0, C.Object(v)) }

// capn.JSON_enabled == false so we stub MarshallJSON().
func (s Z) MarshalJSON() (bs []byte, err error) { return }

type Z_List C.PointerList

func NewZList(s *C.Segment, sz int) Z_List { return Z_List(s.NewCompositeList(16, 1, sz)) }
func (s Z_List) Len() int                  { return C.PointerList(s).Len() }
func (s Z_List) At(i int) Z                { return Z(C.PointerList(s).At(i).ToStruct()) }
func (s Z_List) ToArray() []Z              { return *(*[]Z)(unsafe.Pointer(C.PointerList(s).ToArray())) }
func (s Z_List) Set(i int, item Z)         { C.PointerList(s).Set(i, C.Object(item)) }

type Zgoqserver C.Struct

func NewZgoqserver(s *C.Segment) Zgoqserver       { return Zgoqserver(s.NewStruct(40, 3)) }
func NewRootZgoqserver(s *C.Segment) Zgoqserver   { return Zgoqserver(s.NewRootStruct(40, 3)) }
func AutoNewZgoqserver(s *C.Segment) Zgoqserver   { return Zgoqserver(s.NewStructAR(40, 3)) }
func ReadRootZgoqserver(s *C.Segment) Zgoqserver  { return Zgoqserver(s.Root(0).ToStruct()) }
func (s Zgoqserver) Nextjobid() int64             { return int64(C.Struct(s).Get64(0)) }
func (s Zgoqserver) SetNextjobid(v int64)         { C.Struct(s).Set64(0, uint64(v)) }
func (s Zgoqserver) Runq() Zjob_List              { return Zjob_List(C.Struct(s).GetObject(0)) }
func (s Zgoqserver) SetRunq(v Zjob_List)          { C.Struct(s).SetObject(0, C.Object(v)) }
func (s Zgoqserver) Waitingjobs() Zjob_List       { return Zjob_List(C.Struct(s).GetObject(1)) }
func (s Zgoqserver) SetWaitingjobs(v Zjob_List)   { C.Struct(s).SetObject(1, C.Object(v)) }
func (s Zgoqserver) Finishedjobscount() int64     { return int64(C.Struct(s).Get64(8)) }
func (s Zgoqserver) SetFinishedjobscount(v int64) { C.Struct(s).Set64(8, uint64(v)) }
func (s Zgoqserver) Badsgtcount() int64           { return int64(C.Struct(s).Get64(16)) }
func (s Zgoqserver) SetBadsgtcount(v int64)       { C.Struct(s).Set64(16, uint64(v)) }
func (s Zgoqserver) Cancelledjobcount() int64     { return int64(C.Struct(s).Get64(24)) }
func (s Zgoqserver) SetCancelledjobcount(v int64) { C.Struct(s).Set64(24, uint64(v)) }
func (s Zgoqserver) Badnoncecount() int64         { return int64(C.Struct(s).Get64(32)) }
func (s Zgoqserver) SetBadnoncecount(v int64)     { C.Struct(s).Set64(32, uint64(v)) }
func (s Zgoqserver) Finishedjobs() Zjob_List      { return Zjob_List(C.Struct(s).GetObject(2)) }
func (s Zgoqserver) SetFinishedjobs(v Zjob_List)  { C.Struct(s).SetObject(2, C.Object(v)) }

// capn.JSON_enabled == false so we stub MarshallJSON().
func (s Zgoqserver) MarshalJSON() (bs []byte, err error) { return }

type Zgoqserver_List C.PointerList

func NewZgoqserverList(s *C.Segment, sz int) Zgoqserver_List {
	return Zgoqserver_List(s.NewCompositeList(40, 3, sz))
}
func (s Zgoqserver_List) Len() int            { return C.PointerList(s).Len() }
func (s Zgoqserver_List) At(i int) Zgoqserver { return Zgoqserver(C.PointerList(s).At(i).ToStruct()) }
func (s Zgoqserver_List) ToArray() []Zgoqserver {
	return *(*[]Zgoqserver)(unsafe.Pointer(C.PointerList(s).ToArray()))
}
func (s Zgoqserver_List) Set(i int, item Zgoqserver) { C.PointerList(s).Set(i, C.Object(item)) }
