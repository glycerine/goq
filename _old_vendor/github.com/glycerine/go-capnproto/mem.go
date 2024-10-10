package capn

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
)

var (
	errBufferCall     = errors.New("capn: can't call on a memory buffer")
	ErrInvalidSegment = errors.New("capn: invalid segment id")
	ErrTooMuchData    = errors.New("capn: too much data in stream")
)

type buffer Segment

// NewBuffer creates an expanding single segment buffer. Creating new objects
// will expand the buffer. Data can be nil (or length 0 with some capacity) if
// creating a new session. If parsing an existing segment then data should be
// the segment contents and will not be copied.
func NewBuffer(data []byte) *Segment {
	if uint64(len(data)) > uint64(math.MaxUint32) {
		return nil
	}

	b := &buffer{}
	b.Message = b
	b.Data = data
	return (*Segment)(b)
}

func (b *buffer) NewSegment(minsz int) (*Segment, error) {
	if minsz < 4096 {
		minsz = 4096
	}
	if uint64(len(b.Data)) > uint64(math.MaxUint32)-uint64(minsz) {
		return nil, ErrOverlarge
	}
	b.Data = append(b.Data, make([]byte, minsz)...)
	b.Data = b.Data[:len(b.Data)-minsz]
	return (*Segment)(b), nil
}

func (b *buffer) Lookup(segid uint32) (*Segment, error) {
	if segid == 0 {
		return (*Segment)(b), nil
	} else {
		return nil, ErrInvalidSegment
	}
}

type MultiBuffer struct {
	Segments []*Segment
}

// NewMultiBuffer creates a new multi segment message. Creating new objects
// will try and reuse the buffers available, but will create new ones if there
// is insufficient capacity. When parsing an existing message data should be
// the list of segments. The data buffers will not be copied.
func NewMultiBuffer(data [][]byte) *Segment {
	m := &MultiBuffer{make([]*Segment, len(data))}
	for i, d := range data {
		m.Segments[i] = &Segment{m, d, uint32(i), false}
	}
	if len(data) > 0 {
		return m.Segments[0]
	}
	return &Segment{Message: m, Data: nil, Id: 0xFFFFFFFF, RootDone: false}
}

var (
	MaxSegmentNumber = 1024
	MaxTotalSize     = 1024 * 1024 * 1024
)

func (m *MultiBuffer) NewSegment(minsz int) (*Segment, error) {
	for _, s := range m.Segments {
		if len(s.Data)+minsz <= cap(s.Data) {
			return s, nil
		}
	}

	if minsz < 4096 {
		minsz = 4096
	}
	s := &Segment{m, make([]byte, 0, minsz), uint32(len(m.Segments)), false}
	m.Segments = append(m.Segments, s)
	return s, nil
}

func (m *MultiBuffer) Lookup(segid uint32) (*Segment, error) {
	if uint(segid) < uint(len(m.Segments)) {
		return m.Segments[segid], nil
	} else {
		return nil, ErrInvalidSegment
	}
}

// ReadFromStream reads a non-packed serialized stream from r. buf is used to
// buffer the read contents, can be nil, and is provided so that the buffer
// can be reused between messages. The returned segment is the first segment
// read, which contains the root pointer.
//
// Warning about buf reuse:  It is safer to just pass nil for buf.
// When making multiple calls to ReadFromStream() with the same buf argument, you
// may overwrite the data in a previously returned Segment.
// The re-use of buf is an optimization for when you are actually
// done with any previously returned Segment which may have data still alive
// in buf.
//
func ReadFromStream(r io.Reader, buf *bytes.Buffer) (*Segment, error) {
	if buf == nil {
		buf = new(bytes.Buffer)
	} else {
		buf.Reset()
	}

	if _, err := io.CopyN(buf, r, 4); err != nil {
		return nil, err
	}

	if binary.LittleEndian.Uint32(buf.Bytes()[:]) >= uint32(MaxSegmentNumber) {
		return nil, ErrTooMuchData
	}

	segnum := int(binary.LittleEndian.Uint32(buf.Bytes()[:]) + 1)
	hdrsz := 8*(segnum/2) + 4

	if _, err := io.CopyN(buf, r, int64(hdrsz)); err != nil {
		return nil, err
	}

	total := 0
	for i := 0; i < segnum; i++ {
		sz := binary.LittleEndian.Uint32(buf.Bytes()[4*i+4:])
		if uint64(total)+uint64(sz)*8 > uint64(MaxTotalSize) {
			return nil, ErrTooMuchData
		}
		total += int(sz) * 8
	}

	if _, err := io.CopyN(buf, r, int64(total)); err != nil {
		return nil, err
	}

	hdrv := buf.Bytes()[4 : hdrsz+4]
	datav := buf.Bytes()[hdrsz+4:]

	if segnum == 1 {
		sz := int(binary.LittleEndian.Uint32(hdrv)) * 8
		return NewBuffer(datav[:sz]), nil
	}

	m := &MultiBuffer{make([]*Segment, segnum)}
	for i := 0; i < segnum; i++ {
		sz := int(binary.LittleEndian.Uint32(hdrv[4*i:])) * 8
		m.Segments[i] = &Segment{m, datav[:sz], uint32(i), false}
		datav = datav[sz:]
	}

	return m.Segments[0], nil
}

// ReadFromMemoryZeroCopy: like ReadFromStream, but reads a non-packed
// serialized stream that already resides in memory in the argument data.
// The returned segment is the first segment read, which contains
// the root pointer. The returned bytesRead says how many bytes were
// consumed from data in making seg. The caller should advance the
// data slice by doing data = data[bytesRead:] between successive calls
// to ReadFromMemoryZeroCopy().
func ReadFromMemoryZeroCopy(data []byte) (seg *Segment, bytesRead int64, err error) {

	dataLen := len(data)
	if dataLen < 4 {
		return nil, 0, io.EOF
	}

	if binary.LittleEndian.Uint32(data[0:4]) >= uint32(MaxSegmentNumber) {
		return nil, 0, ErrTooMuchData
	}

	segnum := int(binary.LittleEndian.Uint32(data[0:4]) + 1)
	hdrsz := 8*(segnum/2) + 4

	if dataLen < hdrsz+4 {
		return nil, 0, io.EOF
	}
	b := data[0:(hdrsz + 4)]

	total := 0
	for i := 0; i < segnum; i++ {
		sz := binary.LittleEndian.Uint32(b[4*i+4:])
		if uint64(total)+uint64(sz)*8 > uint64(MaxTotalSize) {
			return nil, 0, ErrTooMuchData
		}
		total += int(sz) * 8
	}
	if total == 0 {
		return nil, 0, io.EOF
	}

	hdrv := data[4:(hdrsz + 4)]
	datav := data[hdrsz+4:]
	m := &MultiBuffer{make([]*Segment, segnum)}
	for i := 0; i < segnum; i++ {
		sz := int(binary.LittleEndian.Uint32(hdrv[4*i:])) * 8
		if len(datav) < sz {
			return nil, 0, io.EOF
		}
		m.Segments[i] = &Segment{m, datav[:sz], uint32(i), false}
		datav = datav[sz:]
	}

	return m.Segments[0], int64(4 + hdrsz + total), nil
}

func NewSingleSegmentMultiBuffer() *MultiBuffer {
	m := &MultiBuffer{make([]*Segment, 1)}
	m.Segments[0] = &Segment{}
	return m
}

// ReadFromMemoryZeroCopyNoAlloc: like ReadFromMemoryZeroCopy,
// but avoid all allocations so we get zero GC pressure.
//
// This requires some strict but easy to meet pre-requisites:
//
// PRE: the capnp bytes in data must come from only one segment. Else we panic.
// PRE: multi must point to an existing MultiBuffer that has exactly one Segment
//      that will be re-used and over-written. If in doubt,
//      you can allocate a correct new one the first time
//      by calling NewSingleSegmentMultiBuffer().
//
func ReadFromMemoryZeroCopyNoAlloc(data []byte, multi *MultiBuffer) (bytesRead int64, err error) {

	if len(data) < 4 {
		return 0, io.EOF
	}

	if binary.LittleEndian.Uint32(data[0:4]) >= uint32(MaxSegmentNumber) {
		return 0, ErrTooMuchData
	}

	segnum := int(binary.LittleEndian.Uint32(data[0:4]) + 1)
	if segnum != 1 {
		panic("only one segment allowed in data read in with ReadFromMemoryZeroCopyNoAlloc()")
	}
	if multi == nil {
		panic("multi must point to an existing MultiBuffer with a single Segment")
	}
	if len(multi.Segments) != 1 {
		panic("only one segment allowed in the multi *MultiBuffer used in ReadFromMemoryZeroCopyNoAlloc()")
	}
	if multi.Segments[0] == nil {
		panic("multi.Segment[0] must point to an allocated Segment{} to be resused in ReadFromMemoryZeroCopyNoAlloc()")
	}

	hdrsz := 8*(segnum/2) + 4

	b := data[0:(hdrsz + 4)]

	total := 0
	for i := 0; i < segnum; i++ {
		sz := binary.LittleEndian.Uint32(b[4*i+4:])
		if uint64(total)+uint64(sz)*8 > uint64(MaxTotalSize) {
			return 0, ErrTooMuchData
		}
		total += int(sz) * 8
	}
	if total == 0 {
		return 0, io.EOF
	}

	hdrv := data[4:(hdrsz + 4)]
	datav := data[hdrsz+4:]
	sz := int(binary.LittleEndian.Uint32(hdrv)) * 8
	seg := multi.Segments[0]
	seg.Message = multi
	seg.Data = datav[:sz]
	seg.Id = 0
	seg.RootDone = false
	datav = datav[sz:]

	return int64(4 + hdrsz + total), nil
}

// WriteTo writes the message that the segment is part of to the
// provided stream in serialized form.
func (s *Segment) WriteTo(w io.Writer) (int64, error) {
	segnum := uint32(1)
	for {
		if seg, _ := s.Message.Lookup(segnum); seg == nil {
			break
		}
		segnum++
	}

	hdrv := make([]uint8, 8*(segnum/2)+8)
	binary.LittleEndian.PutUint32(hdrv, segnum-1)
	for i := uint32(0); i < segnum; i++ {
		seg, _ := s.Message.Lookup(i)
		binary.LittleEndian.PutUint32(hdrv[4*i+4:], uint32(len(seg.Data)/8))
	}

	if n, err := w.Write(hdrv); err != nil {
		return int64(n), err
	}
	written := int64(len(hdrv))

	for i := uint32(0); i < segnum; i++ {
		seg, _ := s.Message.Lookup(i)
		if n, err := w.Write(seg.Data); err != nil {
			return written + int64(n), err
		} else {
			written += int64(n)
		}
	}

	return written, nil
}
