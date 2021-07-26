package http3

import (
	"io"

	"github.com/lucas-clemente/quic-go/quicvarint"
)

type Frame interface {
	FrameType() uint64
}

func ParseFrame(r io.Reader) (Frame, error) {
	qr := quicvarint.NewReader(r)
	frameType, err := quicvarint.Read(qr)
	if err != nil {
		return nil, err
	}
	switch frameType {
	case FrameTypeSettings:
		len, err := quicvarint.Read(qr)
		if err != nil {
			return nil, err
		}
		return ParseSettings(io.LimitReader(r, len))

	}
}
