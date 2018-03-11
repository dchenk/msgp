package msgp_test

//go:generate msgp -o=defs_gen_test.go -tests=false

type Blobs []Blob

type Blob struct {
	Name   string  `msgp:"name"`
	Float  float64 `msgp:"float"`
	Bytes  []byte  `msgp:"bytes"`
	Amount int64   `msgp:"amount"`
}
