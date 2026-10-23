// Package endpoint implements the frozen Herdr generation-one control lane.
package endpoint

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const MaxFrameSize = 2 * 1024 * 1024
const MaxImageSize = 16 * 1024 * 1024

// The variant numbers are frozen by Herdr src/protocol/wire.rs.
const controlTag = 20
const notificationTag = 14
const responseTag = 18

func appendUint(b []byte, n uint64) []byte {
	switch {
	case n < 251:
		return append(b, byte(n))
	case n <= 65535:
		return binary.LittleEndian.AppendUint16(append(b, 251), uint16(n))
	case n <= 4294967295:
		return binary.LittleEndian.AppendUint32(append(b, 252), uint32(n))
	default:
		return binary.LittleEndian.AppendUint64(append(b, 253), n)
	}
}
func appendString(b []byte, s string) []byte { return append(appendUint(b, uint64(len(s))), s...) }
func control(kind, data string) []byte {
	return appendString(appendString([]byte{controlTag}, kind), data)
}

type decoder struct {
	b   []byte
	err error
}

func (d *decoder) take(n uint64) []byte {
	if d.err != nil {
		return nil
	}
	if n > uint64(len(d.b)) {
		d.err = io.ErrUnexpectedEOF
		return nil
	}
	b := d.b[:n]
	d.b = d.b[n:]
	return b
}
func (d *decoder) uint() uint64 {
	b := d.take(1)
	if b == nil {
		return 0
	}
	switch b[0] {
	case 251:
		b = d.take(2)
		if b != nil {
			return uint64(binary.LittleEndian.Uint16(b))
		}
	case 252:
		b = d.take(4)
		if b != nil {
			return uint64(binary.LittleEndian.Uint32(b))
		}
	case 253:
		b = d.take(8)
		if b != nil {
			return binary.LittleEndian.Uint64(b)
		}
	case 254, 255:
		d.err = errors.New("unsupported bincode integer")
	default:
		return uint64(b[0])
	}
	return 0
}
func (d *decoder) str() string {
	b := d.take(d.uint())
	if d.err == nil && !utf8.Valid(b) {
		d.err = errors.New("invalid UTF-8")
	}
	return string(b)
}
func (d *decoder) option() bool {
	b := d.take(1)
	if b == nil {
		return false
	}
	if b[0] > 1 {
		d.err = errors.New("invalid bincode option")
	}
	return b[0] == 1
}
func (d *decoder) optionalString() string {
	if d.option() {
		return d.str()
	}
	return ""
}
func (d *decoder) finish() error {
	if d.err != nil {
		return d.err
	}
	if len(d.b) != 0 {
		return errors.New("trailing payload bytes")
	}
	return nil
}
func readFrame(r io.Reader) ([]byte, error) {
	var h [4]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(h[:])
	if n == 0 || n > MaxFrameSize {
		return nil, fmt.Errorf("endpoint frame size %d exceeds bounds", n)
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}
func writeFrame(w io.Writer, b []byte) error {
	if len(b) == 0 || len(b) > MaxImageSize+1024 {
		return errors.New("endpoint outgoing frame exceeds bounds")
	}
	out := binary.LittleEndian.AppendUint32(nil, uint32(len(b)))
	out = append(out, b...)
	for len(out) > 0 {
		n, err := w.Write(out)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		out = out[n:]
	}
	return nil
}
