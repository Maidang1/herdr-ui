// Package protocol speaks the wire the herdr daemon speaks.
//
// The wire is a stream of frames: each frame is a length of four bytes,
// little endian, and the bytes of the message. The message itself is
// written in the bincode the daemon uses — the "standard" profile of
// bincode 2: little endian, and integers written in the variable form,
// where a number that fits in a byte is one byte, and a larger one is a
// marker byte that says how many bytes follow.
//
// The rules here are the rules of the daemon, and they are checked against
// the bytes the Rust client produces: see golden_test.go.
package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf8"
)

// The markers that say how many bytes a number takes.
const (
	// markerU16 says that a u16 follows.
	markerU16 = 0xFB
	// markerU32 says that a u32 follows.
	markerU32 = 0xFC
	// markerU64 says that a u64 follows.
	markerU64 = 0xFD
	// singleByteMax is the largest number that is written in one byte.
	singleByteMax = 250
)

// ErrTruncated says that the bytes ran out before the message did.
var ErrTruncated = errors.New("protocol: the message ended too early")

// ErrTrailing says that there were bytes left over after the message.
var ErrTrailing = errors.New("protocol: bytes were left over after the message")

// Writer writes the bytes of a message.
type Writer struct {
	buf []byte
}

// Payload returns the bytes written so far.
func (w *Writer) Payload() []byte { return w.buf }

// Len returns the number of bytes written so far.
func (w *Writer) Len() int { return len(w.buf) }

// U8 writes one byte.
func (w *Writer) U8(v uint8) {
	w.buf = append(w.buf, v)
}

// Bool writes a true or a false.
func (w *Writer) Bool(v bool) {
	if v {
		w.buf = append(w.buf, 1)
		return
	}
	w.buf = append(w.buf, 0)
}

// U16 writes a number of two bytes or fewer.
func (w *Writer) U16(v uint16) { w.unsigned(uint64(v)) }

// U32 writes a number of four bytes or fewer.
func (w *Writer) U32(v uint32) { w.unsigned(uint64(v)) }

// U64 writes a number of eight bytes or fewer.
func (w *Writer) U64(v uint64) { w.unsigned(v) }

// unsigned writes a number in the variable form.
//
// A number that fits in a byte is one byte. A number that does not is a
// marker byte that says what follows: two, four or eight bytes, little
// endian.
func (w *Writer) unsigned(v uint64) {
	switch {
	case v <= singleByteMax:
		w.buf = append(w.buf, byte(v))
	case v <= math.MaxUint16:
		w.buf = append(w.buf, markerU16)
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(v))
		w.buf = append(w.buf, b[:]...)
	case v <= math.MaxUint32:
		w.buf = append(w.buf, markerU32)
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32(v))
		w.buf = append(w.buf, b[:]...)
	default:
		w.buf = append(w.buf, markerU64)
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], v)
		w.buf = append(w.buf, b[:]...)
	}
}

// I32 writes a signed number.
func (w *Writer) I32(v int32) { w.unsigned(zigzag(int64(v))) }

// I64 writes a signed number.
func (w *Writer) I64(v int64) { w.unsigned(zigzag(v)) }

// zigzag turns a signed number into an unsigned one that keeps its order:
// -1 becomes 1, 1 becomes 2.
func zigzag(v int64) uint64 {
	if v < 0 {
		return uint64(^v<<1) | 1
	}
	return uint64(v << 1)
}

// unzigzag turns what zigzag made back into a signed number.
func unzigzag(v uint64) int64 {
	return int64(v>>1) ^ -int64(v&1)
}

// Str writes a string: its length, then its bytes.
func (w *Writer) Str(s string) {
	w.unsigned(uint64(len(s)))
	w.buf = append(w.buf, s...)
}

// Bytes writes a run of bytes: its length, then the bytes.
func (w *Writer) Bytes(b []byte) {
	w.unsigned(uint64(len(b)))
	w.buf = append(w.buf, b...)
}

// Rune writes a character as the bytes of its name in UTF-8.
//
// A character takes no length: the daemon writes the characters of a key
// straight, and the reader knows how long a character is.
func (w *Writer) Rune(r rune) {
	var b [4]byte
	n := utf8.EncodeRune(b[:], r)
	w.buf = append(w.buf, b[:n]...)
}

// Count writes the number of things that follow, in the variable form.
//
// It is the length of a list of things, and of a string.
func (w *Writer) Count(n uint64) { w.unsigned(n) }

// Variant writes the number of the case of an enum.
func (w *Writer) Variant(index uint32) { w.unsigned(uint64(index)) }

// Reader reads the bytes of a message.
type Reader struct {
	buf []byte
	off int
}

// NewReader returns a reader over the bytes.
func NewReader(b []byte) *Reader { return &Reader{buf: b} }

// Remaining returns the bytes that are not read yet.
func (r *Reader) Remaining() int { return len(r.buf) - r.off }

// U8 reads one byte.
func (r *Reader) U8() (uint8, error) {
	if r.off+1 > len(r.buf) {
		return 0, ErrTruncated
	}
	v := r.buf[r.off]
	r.off++
	return v, nil
}

// Bool reads a true or a false.
func (r *Reader) Bool() (bool, error) {
	v, err := r.U8()
	if err != nil {
		return false, err
	}
	switch v {
	case 0:
		return false, nil
	case 1:
		return true, nil
	}
	return false, fmt.Errorf("protocol: a true or a false was expected, got %d", v)
}

// U16 reads a number of two bytes or fewer.
func (r *Reader) U16() (uint16, error) {
	v, err := r.unsigned()
	if err != nil {
		return 0, err
	}
	if v > math.MaxUint16 {
		return 0, fmt.Errorf("protocol: the number %d does not fit in two bytes", v)
	}
	return uint16(v), nil
}

// U32 reads a number of four bytes or fewer.
func (r *Reader) U32() (uint32, error) {
	v, err := r.unsigned()
	if err != nil {
		return 0, err
	}
	if v > math.MaxUint32 {
		return 0, fmt.Errorf("protocol: the number %d does not fit in four bytes", v)
	}
	return uint32(v), nil
}

// U64 reads a number of eight bytes or fewer.
func (r *Reader) U64() (uint64, error) { return r.unsigned() }

// unsigned reads a number in the variable form.
func (r *Reader) unsigned() (uint64, error) {
	first, err := r.U8()
	if err != nil {
		return 0, err
	}
	switch {
	case first <= singleByteMax:
		return uint64(first), nil
	case first == markerU16:
		b, err := r.take(2)
		if err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint16(b)), nil
	case first == markerU32:
		b, err := r.take(4)
		if err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint32(b)), nil
	case first == markerU64:
		b, err := r.take(8)
		if err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint64(b), nil
	}
	return 0, fmt.Errorf("protocol: %d is not a number the daemon writes", first)
}

// take reads n bytes.
func (r *Reader) take(n int) ([]byte, error) {
	if r.off+n > len(r.buf) {
		return nil, ErrTruncated
	}
	b := r.buf[r.off : r.off+n]
	r.off += n
	return b, nil
}

// I32 reads a signed number.
func (r *Reader) I32() (int32, error) {
	v, err := r.I64()
	if err != nil {
		return 0, err
	}
	if v > math.MaxInt32 || v < math.MinInt32 {
		return 0, fmt.Errorf("protocol: the number %d does not fit in four bytes", v)
	}
	return int32(v), nil
}

// I64 reads a signed number.
func (r *Reader) I64() (int64, error) {
	v, err := r.unsigned()
	if err != nil {
		return 0, err
	}
	return unzigzag(v), nil
}

// Str reads a string: its length, then its bytes.
func (r *Reader) Str() (string, error) {
	b, err := r.ByteSlice()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ByteSlice reads a run of bytes: its length, then the bytes.
func (r *Reader) ByteSlice() ([]byte, error) {
	length, err := r.unsigned()
	if err != nil {
		return nil, err
	}
	if length > uint64(r.Remaining()) {
		return nil, ErrTruncated
	}
	return r.take(int(length))
}

// Rune reads a character: the bytes of its name in UTF-8, without a
// length. The reader reads one byte at a time, because a character takes
// as many bytes as it takes.
func (r *Reader) Rune() (rune, error) {
	b, err := r.U8()
	if err != nil {
		return 0, err
	}
	if b < 0x80 {
		return rune(b), nil
	}
	var buf [4]byte
	buf[0] = b
	switch {
	case b&0xE0 == 0xC0:
		if err := r.readInto(buf[1:2]); err != nil {
			return 0, err
		}
	case b&0xF0 == 0xE0:
		if err := r.readInto(buf[1:3]); err != nil {
			return 0, err
		}
	case b&0xF8 == 0xF0:
		if err := r.readInto(buf[1:4]); err != nil {
			return 0, err
		}
	default:
		return 0, fmt.Errorf("protocol: %d is not the first byte of a character", b)
	}
	decoded, size := utf8.DecodeRune(buf[:])
	if decoded == utf8.RuneError && size <= 1 {
		return 0, fmt.Errorf("protocol: the bytes are not a character")
	}
	return decoded, nil
}

// readInto fills the bytes given.
func (r *Reader) readInto(dst []byte) error {
	b, err := r.take(len(dst))
	if err != nil {
		return err
	}
	copy(dst, b)
	return nil
}

// Variant reads the number of the case of an enum.
func (r *Reader) Variant() (uint32, error) {
	v, err := r.unsigned()
	if err != nil {
		return 0, err
	}
	if v > math.MaxUint32 {
		return 0, fmt.Errorf("protocol: the number %d is not a case of an enum", v)
	}
	return uint32(v), nil
}
