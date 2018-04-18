package tests

// Because many MessagePack implementations by default encode all non-negative integers, we should
// be able to decode unsigned integers into Go's signed integers when the unsigned does not overflow.

//go:generate msgp -tests=false -io=false
//msgp:marshal ignore MixedIntsDec Int32ForUint16Dec
//msgp:unmarshal ignore MixedIntsEnc Uint16Enc

// MixedIntsEnc values should be decodable into a MixedInts1dec (if no integers overflow or
// are negative).
type MixedIntsEnc struct {
	A int64
	B uint8
	C int32
	D uint64
}

// MixedIntsDec values should be decodable from the encoded MixedIntsEnc type.
type MixedIntsDec struct {
	A int64 // Keeping same as above (like in real world).
	B int8
	C int32
	D int64
}

// Uint16Enc values should be decodable into a Int32ForUint16Dec (if no integers overflow or
// are negative).
type Uint16Enc uint16

// Int32ForUint16Dec values should be decodable from the encoded Uint16Enc type.
type Int32ForUint16Dec int32
