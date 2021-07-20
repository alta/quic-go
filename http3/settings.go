package http3

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/lucas-clemente/quic-go/internal/protocol"
	"github.com/lucas-clemente/quic-go/quicvarint"
)

const (
	FrameTypeSettings = 0x4

	SettingDatagram = 0x276
)

type Settings interface {
	Has(id uint64) bool
	Get(id uint64) uint64
	Set(id, value uint64)
	Delete(id uint64)
	Count() int
	WriteFrame(w io.Writer) error
}

type settings map[uint64]uint64

func NewSettings() Settings {
	return settings{}
}

func (s settings) Has(id uint64) bool {
	_, ok := s[id]
	return ok
}

func (s settings) Get(id uint64) uint64 {
	return s[id]
}

func (s settings) Set(id, value uint64) {
	s[id] = value
}

func (s settings) Delete(id uint64) {
	delete(s, id)
}

func (s settings) Count() int {
	return len(s)
}

func (s settings) WriteFrame(w io.Writer) error {
	quicvarint.Write(w, uint64(s.FrameType()))
	quicvarint.Write(w, uint64(s.FrameLength()))
	ids := make([]uint64, 0, len(s))
	for id := range s {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return i < j })
	for _, id := range ids {
		quicvarint.Write(w, id)
		quicvarint.Write(w, s[id])
	}
	return nil
}

func (s settings) FrameType() uint64 {
	return FrameTypeSettings
}

func (s settings) FrameLength() protocol.ByteCount {
	var len protocol.ByteCount
	for id, val := range s {
		len += quicvarint.Len(id) + quicvarint.Len(val)
	}
	return len
}

func ReadSettingsFrame(r io.Reader, l uint64) (Settings, error) {
	if l > 8*(1<<10) {
		return nil, fmt.Errorf("unexpected size for SETTINGS frame: %d", l)
	}
	buf := make([]byte, l)
	if _, err := io.ReadFull(r, buf); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, io.EOF
		}
		return nil, err
	}
	s := NewSettings()
	b := bytes.NewReader(buf)
	for b.Len() > 0 {
		id, err := quicvarint.Read(b)
		if err != nil { // should not happen. We allocated the whole frame already.
			return nil, err
		}
		val, err := quicvarint.Read(b)
		if err != nil { // should not happen. We allocated the whole frame already.
			return nil, err
		}

		if s.Has(id) {
			return nil, fmt.Errorf("duplicate setting: %d", id)
		}
		s.Set(id, val)
	}
	return s, nil
}
