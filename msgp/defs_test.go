package msgp_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/dchenk/msgp/msgp"
)

//go:generate msgp -o=defs_gen_test.go -tests=false

type Blobs []Blob // needed separately for Msgsize()

type Blob struct {
	Name  string  `msgp:"name"`
	Float float64 `msgp:"float"`
	Inner struct {
		F float32 `msgp:"f32"`
	} `msgp:"inner"`
	Bytes    []byte `msgp:"bytes"`
	Amount   int64  `msgp:"amount"`
	Unsigned uint16 // Use field name
}

var (
	blobStrings  = []string{"", "a string", "a longer string here!"}
	blobFloats   = []float64{0.0, -1.00000007, 1.0, 3.1415926535}
	blobFloats32 = []float32{-34.5243, 0.0, -1.0, 1.0, 3.526258}
	blobIntegers = []int64{0, 1, -1, 80000, 1 << 30}
	blobBytes    = [][]byte{{}, []byte("hello"), []byte(`{"is_json":true, "more_stuff":[75]}`)}
)

// TestEncodeDecode tests Blobs for actual data integrity.
func TestEncodeDecode(t *testing.T) {

	const size = 5
	data := make(Blobs, size)

	for i := range data {
		datum := &data[i]
		datum.Name = blobStrings[rand.Intn(len(blobStrings))]
		datum.Float = blobFloats[rand.Intn(len(blobFloats))]
		datum.Inner.F = blobFloats32[rand.Intn(len(blobFloats32))]
		datum.Amount = blobIntegers[rand.Intn(len(blobIntegers))]
		datum.Bytes = blobBytes[rand.Intn(len(blobBytes))]
	}

	var msg bytes.Buffer

	err := msgp.Encode(&msg, &data)
	if err != nil {
		t.Fatalf("could not encode; %v", err)
	}

	// Each element that we are encoding has five objects (the minimum length
	// of any single thing in MessagePack is one byte).
	if msg.Len() < size*5 {
		t.Fatalf("not enough data encoded")
	}

	var decoded Blobs
	err = msgp.Decode(&msg, &decoded)
	if err != nil {
		t.Fatalf("could not decode; %v", err)
	}

	// Ensure we have five things.
	for i := 0; i < size; i++ {
		datum := data[i]
		if v := decoded[i].Name; datum.Name != v {
			t.Errorf("(index %d) bad Name: %q", i, v)
		}
		if v := decoded[i].Float; datum.Float != v {
			t.Errorf("(index %d) bad Float: %v", i, v)
		}
		if v := decoded[i].Inner.F; datum.Inner.F != v {
			t.Errorf("(index %d) bad Inner.F: %v", i, v)
		}
		if v := decoded[i].Bytes; !bytes.Equal(datum.Bytes, v) {
			t.Errorf("(index %d) bad Bytes: %v", i, v)
		}
		if v := decoded[i].Unsigned; datum.Unsigned != v {
			t.Errorf("(index %d) bad Unsigned: %v", i, v)
		}
	}

}
