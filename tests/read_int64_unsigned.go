package tests

// Because many MessagePack implementations by default encode all non-negative integers, we should
// be able to decode unsigned integers into Go's signed integers when the unsigned does not overflow.

//go:generate msgp -tests=false -io=false
//msgp:marshal ignore MixedInts1_dec Int16ForUint32_dec
//msgp:unmarshal ignore MixedInts1_enc Uint32_enc

// An encoded MixedInts1_enc should be decodable into a MixedInts1dec (if no integers overflow or
// are negative).
type MixedInts_enc struct {
	A int64
	B uint8
	C int32
	D uint64
}

type MixedInts_dec struct {
	A int64 // Keeping same as above (like in real world).
	B int8
	C int32
	D int64
}

// An encoded Uint16_enc should be decodable into a Int32ForUint16_dec.
type Uint16_enc uint16

type Int32ForUint16_dec int32
