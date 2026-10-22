package bincode

import (
	"encoding/hex"
	"testing"
)

// The bytes here come from the Rust implementation of the protocol, so that
// a change in this package that breaks the wire shows up as a failing test
// rather than as a daemon that hangs up.
func TestNumbersMatchTheRustImplementation(t *testing.T) {
	// The encoding is canonical: a number is written in the fewest bytes
	// that hold it, so reading a wider form and writing it again gives the
	// narrower one.
	cases := []struct {
		name  string
		got   string
		want  string
		value uint64
	}{
		{"small", "00", "00", 0},
		{"one byte", "7d", "7d", 125},
		{"the largest single byte", "fa", "fa", 250},
		{"two bytes", "fb2c01", "fb2c01", 300},
		{"four bytes", "fc2c010000", "fb2c01", 300},
		{"eight bytes", "fd2c01000000000000", "fb2c01", 300},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			value, err := hex.DecodeString(test.got)
			if err != nil {
				t.Fatal(err)
			}
			decoder := NewDecoder(value)
			number, err := decoder.U64()
			if err != nil {
				t.Fatal(err)
			}
			if !decoder.Done() {
				t.Fatalf("%d bytes left over", decoder.Remaining())
			}
			if number != test.value {
				t.Fatalf("the number came back as %d, want %d", number, test.value)
			}
			encoder := NewEncoder()
			encoder.U64(number)
			if got := hex.EncodeToString(encoder.Bytes()); got != test.want {
				t.Fatalf("the number came out as %s, want %s", got, test.want)
			}
		})
	}
}

func TestSignedNumbersUseZigzag(t *testing.T) {
	cases := []struct {
		value int64
		want  string
	}{
		{0, "00"},
		{1, "02"},
		{-1, "01"},
		{2, "04"},
		{-2, "03"},
	}
	for _, test := range cases {
		encoder := NewEncoder()
		encoder.I64(test.value)
		if got := hex.EncodeToString(encoder.Bytes()); got != test.want {
			t.Fatalf("%d came out as %s, want %s", test.value, got, test.want)
		}
		decoder := NewDecoder(encoder.Bytes())
		value, err := decoder.I64()
		if err != nil {
			t.Fatal(err)
		}
		if value != test.value {
			t.Fatalf("%d came back as %d", test.value, value)
		}
	}
}

func TestStringsCarryTheirLength(t *testing.T) {
	encoder := NewEncoder()
	encoder.String("endpoint.hello.v1")
	want := "11656e64706f696e742e68656c6c6f2e7631"
	if got := hex.EncodeToString(encoder.Bytes()); got != want {
		t.Fatalf("the string came out as %s, want %s", got, want)
	}
	decoder := NewDecoder(encoder.Bytes())
	value, err := decoder.String()
	if err != nil {
		t.Fatal(err)
	}
	if value != "endpoint.hello.v1" {
		t.Fatalf("the string came back as %q", value)
	}
}

func TestOptionsCarryTheirTag(t *testing.T) {
	encoder := NewEncoder()
	encoder.Option(false)
	encoder.Option(true)
	if got := hex.EncodeToString(encoder.Bytes()); got != "0001" {
		t.Fatalf("the tags came out as %s, want 0001", got)
	}
	decoder := NewDecoder(encoder.Bytes())
	present, err := decoder.Option()
	if err != nil {
		t.Fatal(err)
	}
	if present {
		t.Fatal("the first option should be empty")
	}
	present, err = decoder.Option()
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("the second option should hold a value")
	}
}

func TestVectorsCarryTheirCount(t *testing.T) {
	encoder := NewEncoder()
	encoder.U64(3)
	encoder.U64(1)
	encoder.U64(2)
	encoder.U64(3)
	decoder := NewDecoder(encoder.Bytes())
	count, err := decoder.U64()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("the count came back as %d", count)
	}
	for i := uint64(1); i <= 3; i++ {
		value, err := decoder.U64()
		if err != nil {
			t.Fatal(err)
		}
		if value != i {
			t.Fatalf("the item came back as %d, want %d", value, i)
		}
	}
	if !decoder.Done() {
		t.Fatal("the decoder has bytes left over")
	}
}

func TestDecoderStopsAtTheEnd(t *testing.T) {
	// A marker that says a number follows, with nothing after it.
	decoder := NewDecoder([]byte{0xfb})
	if _, err := decoder.U64(); err == nil {
		t.Fatal("the decoder should have run out")
	}
}

func TestFloatsKeepTheirBytes(t *testing.T) {
	encoder := NewEncoder()
	encoder.F64(1.5)
	if got := hex.EncodeToString(encoder.Bytes()); got != "000000000000f83f" {
		t.Fatalf("the float came out as %s", got)
	}
	decoder := NewDecoder(encoder.Bytes())
	value, err := decoder.F64()
	if err != nil {
		t.Fatal(err)
	}
	if value != 1.5 {
		t.Fatalf("the float came back as %v", value)
	}
}

func TestSkipMovesOverBytes(t *testing.T) {
	decoder := NewDecoder([]byte{1, 2, 3})
	if err := decoder.Skip(2); err != nil {
		t.Fatal(err)
	}
	value, err := decoder.U8()
	if err != nil {
		t.Fatal(err)
	}
	if value != 3 {
		t.Fatalf("the byte came back as %d", value)
	}
	if !decoder.Done() {
		t.Fatalf("the decoder has %d bytes left over", decoder.Remaining())
	}
}
