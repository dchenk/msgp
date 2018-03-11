package msgp_test

import (
	"bytes"
	"crypto/rand"
	prand "math/rand"
	"os"
	"testing"

	"github.com/dchenk/msgp/msgp"
)

type rawBytes []byte

func (r rawBytes) MarshalMsg(b []byte) ([]byte, error) {
	return msgp.AppendBytes(b, []byte(r)), nil
}

func (r rawBytes) Msgsize() int {
	return msgp.BytesPrefixSize + len(r)
}

func (r *rawBytes) UnmarshalMsg(b []byte) ([]byte, error) {
	tmp, out, err := msgp.ReadBytesBytes(b, (*(*[]byte)(r))[:0])
	*r = rawBytes(tmp)
	return out, err
}

func TestReadWriteFile(t *testing.T) {

	t.Parallel()

	fname := "tmpfile"
	f, err := os.Create(fname)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		f.Close()
		os.Remove(fname)
	}()

	data := make([]byte, 1024*1024)
	if _, err = rand.Read(data); err != nil {
		t.Fatalf("rand reader: %v", err)
	}

	if err = msgp.WriteFile(rawBytes(data), f); err != nil {
		t.Fatalf("writing file: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("could not close file; %v", err)
	}

	f, err = os.Open(fname)
	if err != nil {
		t.Fatalf("could not open written file; %v", err)
	}

	var out rawBytes
	if err = msgp.ReadFile(&out, f); err != nil {
		t.Fatalf("reading file: %v", err)
	}

	if !bytes.Equal([]byte(out), data) {
		t.Fatal("Input not equal to output.")
	}

}

var blobstrings = []string{"", "a string", "a longer string here!"}
var blobfloats = []float64{0.0, -1.0, 1.0, 3.1415926535}
var blobints = []int64{0, 1, -1, 80000, 1 << 30}
var blobbytes = [][]byte{{}, []byte("hello"), []byte(`{"is_json":true, "is_compact":"unable to determine"}`)}

func BenchmarkWriteReadFile(b *testing.B) {

	// Let's not run out of disk space.
	if b.N > 10000000 {
		b.N = 10000000
	}

	fname := "bench-tmpfile"
	f, err := os.Create(fname)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		f.Close()
		os.Remove(fname)
	}()

	data := make(Blobs, b.N)

	for i := range data {
		data[i].Name = blobstrings[prand.Intn(len(blobstrings))]
		data[i].Float = blobfloats[prand.Intn(len(blobfloats))]
		data[i].Amount = blobints[prand.Intn(len(blobints))]
		data[i].Bytes = blobbytes[prand.Intn(len(blobbytes))]
	}

	b.SetBytes(int64(data.Msgsize() / b.N))
	b.ResetTimer()

	err = msgp.WriteFile(data, f)
	if err != nil {
		b.Fatal(err)
	}
	err = msgp.ReadFile(&data, f)
	if err != nil {
		b.Fatal(err)
	}

}
