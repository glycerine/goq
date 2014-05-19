package goq

// AUTO GENERATED - DO NOT EDIT

import (
	C "github.com/glycerine/go-capnproto"
	"unsafe"
)

type JobMsg uint16

const (
	JOBMSG_INITIALSUBMIT     JobMsg = 0
	JOBMSG_ACKSUBMIT                = 1
	JOBMSG_REQUESTFORWORK           = 2
	JOBMSG_DELEGATETOWORKER         = 3
	JOBMSG_SHUTDOWNWORKER           = 4
	JOBMSG_ACKSHUTDOWNWORKER        = 5
	JOBMSG_FINISHEDWORK             = 6
	JOBMSG_ACKFINISHED              = 7
	JOBMSG_SHUTDOWNSERV             = 8
	JOBMSG_ACKSHUTDOWNSERV          = 9
	JOBMSG_CANCELWIP                = 10
	JOBMSG_ACKCANCELWIP             = 11
	JOBMSG_CANCELSUBMIT             = 12
	JOBMSG_ACKCANCELSUBMIT          = 13
	JOBMSG_TAKESNAPSHOT             = 14
	JOBMSG_ACKTAKESNAPSHOT          = 15
	JOBMSG_RESUBMITNOACK            = 16
	JOBMSG_REJECTBADSIG             = 17
	JOBMSG_OBSERVEJOBFINISH         = 18
	JOBMSG_JOBFINISHEDNOTICE        = 19
	JOBMSG_JOBNOTKNOWN              = 20
	JOBMSG_IMMOLATEAWORKERS         = 21
	JOBMSG_IMMOLATEACK              = 22
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
	default:
		return ""
	}
}

type JobMsg_List C.PointerList

func NewJobMsgList(s *C.Segment, sz int) JobMsg_List { return JobMsg_List(s.NewUInt16List(sz)) }
func (s JobMsg_List) Len() int                       { return C.UInt16List(s).Len() }
func (s JobMsg_List) At(i int) JobMsg                { return JobMsg(C.UInt16List(s).At(i)) }
func (s JobMsg_List) ToArray() []JobMsg {
	return *(*[]JobMsg)(unsafe.Pointer(C.UInt16List(s).ToEnumArray()))
}

// capn.JSON_enabled == false so we stub MarshallJSON until List(List(Z)) support is fixed
func (s JobMsg) MarshalJSON() (bs []byte, err error) { return }

type Zjob C.Struct

func NewZjob(s *C.Segment) Zjob           { return Zjob(s.NewStruct(80, 12)) }
func NewRootZjob(s *C.Segment) Zjob       { return Zjob(s.NewRootStruct(80, 12)) }
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

// capn.JSON_enabled == false so we stub MarshallJSON until List(List(Z)) support is fixed
func (s Zjob) MarshalJSON() (bs []byte, err error) { return }

type Zjob_List C.PointerList

func NewZjobList(s *C.Segment, sz int) Zjob_List { return Zjob_List(s.NewCompositeList(80, 12, sz)) }
func (s Zjob_List) Len() int                     { return C.PointerList(s).Len() }
func (s Zjob_List) At(i int) Zjob                { return Zjob(C.PointerList(s).At(i).ToStruct()) }
func (s Zjob_List) ToArray() []Zjob              { return *(*[]Zjob)(unsafe.Pointer(C.PointerList(s).ToArray())) }

type Z C.Struct
type Z_Which uint16

const (
	Z_NOTHING Z_Which = 0
	Z_JOB             = 1
)

func NewZ(s *C.Segment) Z      { return Z(s.NewStruct(16, 1)) }
func NewRootZ(s *C.Segment) Z  { return Z(s.NewRootStruct(16, 1)) }
func ReadRootZ(s *C.Segment) Z { return Z(s.Root(0).ToStruct()) }
func (s Z) Which() Z_Which     { return Z_Which(C.Struct(s).Get16(8)) }
func (s Z) Nothing() int64     { return int64(C.Struct(s).Get64(0)) }
func (s Z) SetNothing(v int64) { C.Struct(s).Set16(8, 0); C.Struct(s).Set64(0, uint64(v)) }
func (s Z) Job() Zjob          { return Zjob(C.Struct(s).GetObject(0).ToStruct()) }
func (s Z) SetJob(v Zjob)      { C.Struct(s).Set16(8, 1); C.Struct(s).SetObject(0, C.Object(v)) }

// capn.JSON_enabled == false so we stub MarshallJSON until List(List(Z)) support is fixed
func (s Z) MarshalJSON() (bs []byte, err error) { return }

type Z_List C.PointerList

func NewZList(s *C.Segment, sz int) Z_List { return Z_List(s.NewCompositeList(16, 1, sz)) }
func (s Z_List) Len() int                  { return C.PointerList(s).Len() }
func (s Z_List) At(i int) Z                { return Z(C.PointerList(s).At(i).ToStruct()) }
func (s Z_List) ToArray() []Z              { return *(*[]Z)(unsafe.Pointer(C.PointerList(s).ToArray())) }
