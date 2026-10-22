// Package bincode encodes and decodes the subset of the bincode 2 standard
// configuration that the Herdr wire model uses.
//
// The standard configuration is little endian and tagged: an unsigned
// integer or a length is one byte while it fits in 250, and otherwise a
// marker byte says how many bytes follow (251 for u16, 252 for u32, 253 for
// u64, 254 for u128). A signed integer is written the way zigzag writes it,
// so that small negative numbers stay short. A float keeps its eight bytes,
// a string or a vector starts with its length, an option starts with a byte
// that says whether it holds a value, and an enum starts with the number of
// its variant. Field order is the order the struct declares its fields,
// which is why the wire model treats it as a compatibility contract.
package bincode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ErrUnexpectedEnd means a frame ended in the middle of a value.
var ErrUnexpectedEnd = errors.New("bincode: unexpected end of input")

// ErrTrailingBytes means a frame held more than the value it carried.
var ErrTrailingBytes = errors.New("bincode: trailing bytes")

// Encoder writes bincode values to a buffer.
type Encoder struct {
	buf []byte
}

// NewEncoder returns an encoder writing into its own buffer.
func NewEncoder() *Encoder { return &Encoder{} }

// Bytes returns what was written.
func (e *Encoder) Bytes() []byte { return e.buf }

// Len returns how many bytes were written.
func (e *Encoder) Len() int { return len(e.buf) }

// U8 writes one byte.
func (e *Encoder) U8(v uint8) { e.buf = append(e.buf, v) }

// Bool writes a boolean.
func (e *Encoder) Bool(v bool) {
	if v {
		e.U8(1)
		return
	}
	e.U8(0)
}

// U16 writes a varint u16.
func (e *Encoder) U16(v uint16) { e.uvarint(uint64(v)) }

// U32 writes a varint u32.
func (e *Encoder) U32(v uint32) { e.uvarint(uint64(v)) }

// U64 writes a varint u64.
func (e *Encoder) U64(v uint64) { e.uvarint(v) }

// I32 writes a zigzag varint i32.
func (e *Encoder) I32(v int32) { e.uvarint(zigzag32(v)) }

// I64 writes a zigzag varint i64.
func (e *Encoder) I64(v int64) { e.uvarint(zigzag64(v)) }

// F64 writes a float as eight little endian bytes.
func (e *Encoder) F64(v float64) {
	e.buf = binary.LittleEndian.AppendUint64(e.buf, math.Float64bits(v))
}

// String writes a string with its length.
func (e *Encoder) String(v string) {
	e.uvarint(uint64(len(v)))
	e.buf = append(e.buf, v...)
}

// BytesOf writes a byte slice with its length.
func (e *Encoder) BytesOf(v []byte) {
	e.uvarint(uint64(len(v)))
	e.buf = append(e.buf, v...)
}

// Option writes the tag of an option and, when it holds one, its value.
func (e *Encoder) Option(present bool) {
	if present {
		e.U8(1)
		return
	}
	e.U8(0)
}

// Variant writes the number of an enum variant.
func (e *Encoder) Variant(v uint32) { e.uvarint(uint64(v)) }

// The markers bincode uses to say how wide a number is.
const (
	singleByteMax = 250
	markerU16     = 251
	markerU32     = 252
	markerU64     = 253
	markerU128    = 254
)

func (e *Encoder) uvarint(v uint64) {
	switch {
	case v <= singleByteMax:
		e.buf = append(e.buf, byte(v))
	case v <= 0xffff:
		e.buf = append(e.buf, markerU16)
		e.U16Fixed(uint16(v))
	case v <= 0xffffffff:
		e.buf = append(e.buf, markerU32)
		e.U32Fixed(uint32(v))
	default:
		e.buf = append(e.buf, markerU64)
		e.U64Fixed(v)
	}
}

// U16Fixed writes a u16 in its plain little endian form.
func (e *Encoder) U16Fixed(v uint16) { e.buf = binary.LittleEndian.AppendUint16(e.buf, v) }

// U32Fixed writes a u32 in its plain little endian form.
func (e *Encoder) U32Fixed(v uint32) { e.buf = binary.LittleEndian.AppendUint32(e.buf, v) }

// U64Fixed writes a u64 in its plain little endian form.
func (e *Encoder) U64Fixed(v uint64) { e.buf = binary.LittleEndian.AppendUint64(e.buf, v) }

func zigzag32(v int32) uint64 { return uint64(uint32(v<<1) ^ uint32(v>>31)) }
func zigzag64(v int64) uint64 { return uint64(v<<1) ^ uint64(v>>63) }

// Decoder reads bincode values from a buffer.
type Decoder struct {
	buf []byte
	pos int
}

// NewDecoder returns a decoder reading from buf.
func NewDecoder(buf []byte) *Decoder { return &Decoder{buf: buf} }

// Remaining returns how many bytes are left.
func (d *Decoder) Remaining() int { return len(d.buf) - d.pos }

// Done reports whether the whole buffer was read.
func (d *Decoder) Done() bool { return d.pos == len(d.buf) }

func (d *Decoder) take(n int) ([]byte, error) {
	if n < 0 || d.Remaining() < n {
		return nil, ErrUnexpectedEnd
	}
	out := d.buf[d.pos : d.pos+n]
	d.pos += n
	return out, nil
}

// Skip moves past n bytes.
func (d *Decoder) Skip(n int) error {
	_, err := d.take(n)
	return err
}

// U8 reads one byte.
func (d *Decoder) U8() (uint8, error) {
	b, err := d.take(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

// Bool reads a boolean.
func (d *Decoder) Bool() (bool, error) {
	v, err := d.U8()
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

// U16 reads a varint u16.
func (d *Decoder) U16() (uint16, error) {
	v, err := d.uvarint()
	return uint16(v), err
}

// U32 reads a varint u32.
func (d *Decoder) U32() (uint32, error) {
	v, err := d.uvarint()
	return uint32(v), err
}

// U64 reads a varint u64.
func (d *Decoder) U64() (uint64, error) { return d.uvarint() }

// I32 reads a zigzag varint i32.
func (d *Decoder) I32() (int32, error) {
	v, err := d.uvarint()
	if err != nil {
		return 0, err
	}
	return int32(v>>1) ^ -int32(v&1), nil
}

// I64 reads a zigzag varint i64.
func (d *Decoder) I64() (int64, error) {
	v, err := d.uvarint()
	if err != nil {
		return 0, err
	}
	return int64(v>>1) ^ -int64(v&1), nil
}

// F64 reads a float.
func (d *Decoder) F64() (float64, error) {
	b, err := d.take(8)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(b)), nil
}

// String reads a string.
func (d *Decoder) String() (string, error) {
	n, err := d.uvarint()
	if err != nil {
		return "", err
	}
	if n > uint64(d.Remaining()) {
		return "", ErrUnexpectedEnd
	}
	b, err := d.take(int(n))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// BytesOf reads a byte slice.
func (d *Decoder) BytesOf() ([]byte, error) {
	n, err := d.uvarint()
	if err != nil {
		return nil, err
	}
	if n > uint64(d.Remaining()) {
		return nil, ErrUnexpectedEnd
	}
	b, err := d.take(int(n))
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

// Option reads the tag of an option.
func (d *Decoder) Option() (bool, error) {
	v, err := d.U8()
	if err != nil {
		return false, err
	}
	switch v {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("bincode: %w: option tag %d", ErrUnexpectedEnd, v)
	}
}

// Variant reads the number of an enum variant.
func (d *Decoder) Variant() (uint32, error) {
	v, err := d.uvarint()
	return uint32(v), err
}

func (d *Decoder) uvarint() (uint64, error) {
	marker, err := d.U8()
	if err != nil {
		return 0, err
	}
	switch {
	case marker <= singleByteMax:
		return uint64(marker), nil
	case marker == markerU16:
		// The bytes after a marker are the plain little endian number.
		b, err := d.take(2)
		if err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint16(b)), nil
	case marker == markerU32:
		b, err := d.take(4)
		if err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint32(b)), nil
	case marker == markerU64:
		b, err := d.take(8)
		if err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint64(b), nil
	case marker == markerU128:
		high, err := d.take(8)
		if err != nil {
			return 0, err
		}
		low, err := d.take(8)
		if err != nil {
			return 0, err
		}
		// A number that needs more than 64 bits does not fit in this
		// client, and the protocol never sends one.
		if binary.LittleEndian.Uint64(high) != 0 {
			return 0, fmt.Errorf("bincode: %w: the number is too large", ErrUnexpectedEnd)
		}
		return binary.LittleEndian.Uint64(low), nil
	default:
		return 0, fmt.Errorf("bincode: %w: unknown number marker %d", ErrUnexpectedEnd, marker)
	}
}
