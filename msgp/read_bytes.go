package msgp

import (
	"bytes"
	"encoding/binary"
	"math"
	"time"
)

var big = binary.BigEndian

// NextType returns the type of the next object in the slice. If the length of the input is zero,
// it returns InvalidType.
func NextType(b []byte) Type {
	if len(b) == 0 {
		return InvalidType
	}
	spec := sizes[b[0]]
	t := spec.typ
	if t == ExtensionType && len(b) > int(spec.size) {
		var tp int8
		if spec.extra == constsize {
			tp = int8(b[1])
		} else {
			tp = int8(b[spec.size-1])
		}
		switch tp {
		case TimeExtension:
			return TimeType
		case Complex128Extension:
			return Complex128Type
		case Complex64Extension:
			return Complex64Type
		default:
			return ExtensionType
		}
	}
	return t
}

// IsNil returns true if len(b)>0 and the leading byte is a "nil" MessagePack byte (0xc0).
func IsNil(b []byte) bool {
	return len(b) > 0 && b[0] == mnil
}

// Raw is raw encoded MessagePack. It implements Marshaler, Unmarshaler, Encoder, Decoder, and Sizer.
// Raw allows you to read and write data without interpreting the contents.
type Raw []byte

// MarshalMsg implements msgp.Marshaler. It appends the raw contents of r to the provided byte slice.
// If r is empty, then "nil" (0xc0) is appended instead.
func (r Raw) MarshalMsg(b []byte) ([]byte, error) {
	i := len(r)
	if i == 0 {
		return AppendNil(b), nil
	}
	o, l := ensure(b, i)
	copy(o[l:], []byte(r))
	return o, nil
}

// UnmarshalMsg implements msgp.Unmarshaler. It sets the contents of r to be the next object in the
// provided byte slice.
func (r *Raw) UnmarshalMsg(b []byte) ([]byte, error) {
	l := len(b)
	out, err := Skip(b)
	if err != nil {
		return b, err
	}
	rlen := l - len(out)
	if cap(*r) < rlen {
		*r = make(Raw, rlen)
	} else {
		*r = (*r)[0:rlen]
	}
	copy(*r, b[:rlen])
	return out, nil
}

// EncodeMsg implements msgp.Encoder. It writes the raw bytes to the writer. If r is empty, then "nil" (0xc0) is
// written instead.
func (r Raw) EncodeMsg(w *Writer) error {
	if len(r) == 0 {
		return w.WriteNil()
	}
	_, err := w.Write([]byte(r))
	return err
}

// DecodeMsg implements msgp.Decoder. It sets the value of r to be the next object on the wire.
func (r *Raw) DecodeMsg(f *Reader) error {
	*r = (*r)[:0]
	return appendNext(f, (*[]byte)(r))
}

// Msgsize implements msgp.Sizer
func (r Raw) Msgsize() int {
	l := len(r)
	if l == 0 {
		return 1 // for 'nil'
	}
	return l
}

func appendNext(f *Reader, d *[]byte) error {
	amt, o, err := getNextSize(f.R)
	if err != nil {
		return err
	}
	var i int
	*d, i = ensure(*d, int(amt))
	_, err = f.R.ReadFull((*d)[i:])
	if err != nil {
		return err
	}
	for o > 0 {
		err = appendNext(f, d)
		if err != nil {
			return err
		}
		o--
	}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (r *Raw) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	_, err := UnmarshalAsJSON(&buf, []byte(*r))
	return buf.Bytes(), err
}

// ReadMapHeaderBytes reads a map header size from b and returns the remaining bytes.
// Possible errors are ErrShortBytes and TypeError.
func ReadMapHeaderBytes(b []byte) (uint32, []byte, error) {
	l := len(b)
	if l < 1 {
		return 0, b, ErrShortBytes
	}

	lead := b[0]
	if isfixmap(lead) {
		return uint32(rfixmap(lead)), b[1:], nil
	}

	switch lead {
	case mmap16:
		if l < 3 {
			return 0, b, ErrShortBytes
		}
		return uint32(big.Uint16(b[1:])), b[3:], nil
	case mmap32:
		if l < 5 {
			return 0, b, ErrShortBytes
		}
		return big.Uint32(b[1:]), b[5:], nil
	default:
		return 0, b, badPrefix(MapType, lead)
	}
}

// ReadMapKeyZC reads a 'str' or 'bin' object (a key to a map element) from b and returns the value
// and any remaining bytes. Possible errors are ErrShortBytes and TypeError.
func ReadMapKeyZC(b []byte) ([]byte, []byte, error) {
	o, b, err := ReadStringZC(b)
	if err != nil {
		if tperr, ok := err.(TypeError); ok && tperr.Encoded == BinType {
			return ReadBytesZC(b)
		}
	}
	return o, b, err
}

// ReadArrayHeaderBytes reads the array header size off of b and returns the array length and
// any remaining bytes. Possible errors are ErrShortBytes, TypeError, and InvalidPrefixError.
func ReadArrayHeaderBytes(b []byte) (uint32, []byte, error) {
	if len(b) < 1 {
		return 0, nil, ErrShortBytes
	}
	lead := b[0]
	if isfixarray(lead) {
		return uint32(rfixarray(lead)), b[1:], nil
	}
	switch lead {
	case marray16:
		if len(b) < 3 {
			return 0, b, ErrShortBytes
		}
		return uint32(big.Uint16(b[1:])), b[3:], nil
	case marray32:
		if len(b) < 5 {
			return 0, b, ErrShortBytes
		}
		return big.Uint32(b[1:]), b[5:], nil
	default:
		return 0, b, badPrefix(ArrayType, lead)
	}
}

// ReadNilBytes reads a "nil" byte off of b and returns any remaining bytes.
// Possible errors are ErrShortBytes, TypeError, and InvalidPrefixError.
func ReadNilBytes(b []byte) ([]byte, error) {
	if len(b) < 1 {
		return nil, ErrShortBytes
	}
	if b[0] != mnil {
		return b, badPrefix(NilType, b[0])
	}
	return b[1:], nil
}

// ReadFloat64Bytes reads a float64 from b and returns the value and any remaining bytes.
// Possible errors are ErrShortBytes and TypeError.
func ReadFloat64Bytes(b []byte) (float64, []byte, error) {
	if len(b) < 5 { // 5 because we may read a float32
		return 0, b, ErrShortBytes
	}
	if b[0] != mfloat64 {
		if b[0] == mfloat32 {
			f32, b, err := ReadFloat32Bytes(b)
			return float64(f32), b, err
		}
		return 0, b, badPrefix(Float64Type, b[0])
	}
	return math.Float64frombits(getMuint64(b)), b[9:], nil
}

// ReadFloat32Bytes reads a float32 from b and returns the value and any remaining bytes.
// Possible errors are ErrShortBytes and TypeError.
func ReadFloat32Bytes(b []byte) (float32, []byte, error) {
	if len(b) < 5 {
		return 0, b, ErrShortBytes
	}
	if b[0] != mfloat32 {
		return 0, b, TypeError{Method: Float32Type, Encoded: getType(b[0])}
	}
	return math.Float32frombits(getMuint32(b)), b[5:], nil
}

// ReadBoolBytes tries to read a float64 from b and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a bool)
func ReadBoolBytes(b []byte) (bool, []byte, error) {
	if len(b) < 1 {
		return false, b, ErrShortBytes
	}
	switch b[0] {
	case mtrue:
		return true, b[1:], nil
	case mfalse:
		return false, b[1:], nil
	default:
		return false, b, badPrefix(BoolType, b[0])
	}
}

// ReadInt64Bytes reads an int64 from b and return the value and the remaining bytes.
// Errors that can be returned are ErrShortBytes, UintOverflow, InvalidPrefixError, and TypeError.
func ReadInt64Bytes(b []byte) (int64, []byte, error) {

	l := len(b)
	if l < 1 {
		return 0, nil, ErrShortBytes
	}

	lead := b[0]
	if isfixint(lead) {
		return int64(rfixint(lead)), b[1:], nil
	}
	if isnfixint(lead) {
		return int64(rnfixint(lead)), b[1:], nil
	}

	switch lead {
	case mint8, muint8:
		if l < 2 {
			return 0, b, ErrShortBytes
		}
		if lead == mint8 {
			return int64(getMint8(b)), b[2:], nil
		}
		return int64(getMuint8(b)), b[2:], nil
	case mint16, muint16:
		if l < 3 {
			return 0, b, ErrShortBytes
		}
		if lead == mint16 {
			return int64(getMint16(b)), b[3:], nil
		}
		return int64(getMuint16(b)), b[3:], nil
	case mint32, muint32:
		if l < 5 {
			return 0, b, ErrShortBytes
		}
		if lead == mint32 {
			return int64(getMint32(b)), b[5:], nil
		}
		return int64(getMint32(b)), b[5:], nil
	case mint64, muint64:
		if l < 9 {
			return 0, b, ErrShortBytes
		}
		if lead == mint64 {
			return getMint64(b), b[9:], nil
		}
		num := getMuint64(b)
		// Only checking for overflow with uint64 because all other (smaller) unsigned
		// integers can fit into an int64.
		if num > math.MaxInt64 {
			return 0, b, UintOverflow{num, 64}
		}
		return int64(num), b[9:], nil
	}

	return 0, b, badPrefix(IntType, lead)

}

// ReadInt32Bytes reads an int32 from b and returns the value and any remaining bytes.
// Possible errors include ErrShortBytes (too few bytes in b), TypeError{} (not an int), and
// IntOverflow{} (value doesn't fit in an int32).
func ReadInt32Bytes(b []byte) (int32, []byte, error) {
	i, o, err := ReadInt64Bytes(b)
	if i > math.MaxInt32 || i < math.MinInt32 {
		return 0, o, IntOverflow{Value: i, FailedBitsize: 32}
	}
	return int32(i), o, err
}

// ReadInt16Bytes reads an int16 from b and returns the value and any remaining bytes.
// Possible errors include ErrShortBytes (too few bytes in b), TypeError{} (not an int), and
// IntOverflow{} (value doesn't fit in an int16).
func ReadInt16Bytes(b []byte) (int16, []byte, error) {
	i, o, err := ReadInt64Bytes(b)
	if i > math.MaxInt16 || i < math.MinInt16 {
		return 0, o, IntOverflow{Value: i, FailedBitsize: 16}
	}
	return int16(i), o, err
}

// ReadInt8Bytes tries to read an int16 from b and return the value and the remaining bytes.
// Possible errors include ErrShortBytes (too few bytes), TypeError{} (not an int), and
// IntOverflow{} (value doesn't fit in an int8).
func ReadInt8Bytes(b []byte) (int8, []byte, error) {
	i, o, err := ReadInt64Bytes(b)
	if i > math.MaxInt8 || i < math.MinInt8 {
		return 0, o, IntOverflow{Value: i, FailedBitsize: 8}
	}
	return int8(i), o, err
}

// ReadIntBytes tries to read an int from b and return the value and the remaining bytes.
// Possible errors include ErrShortBytes (too few bytes), TypeError{} (not a an int), and
// IntOverflow{} (value doesn't fit in an int; 32-bit platforms only).
func ReadIntBytes(b []byte) (int, []byte, error) {
	if smallint {
		i, b, err := ReadInt32Bytes(b)
		return int(i), b, err
	}
	i, b, err := ReadInt64Bytes(b)
	return int(i), b, err
}

// ReadUint64Bytes reads a uint64 from b and returns the value and any remaining bytes.
// Possible errors include ErrShortBytes and TypeError.
func ReadUint64Bytes(b []byte) (uint64, []byte, error) {

	l := len(b)
	if l < 1 {
		return 0, nil, ErrShortBytes
	}

	lead := b[0]
	if isfixint(lead) {
		return uint64(rfixint(lead)), b[1:], nil
	}

	switch lead {
	case muint8:
		if l < 2 {
			return 0, b, ErrShortBytes
		}
		return uint64(getMuint8(b)), b[2:], nil

	case muint16:
		if l < 3 {
			return 0, b, ErrShortBytes
		}
		return uint64(getMuint16(b)), b[3:], nil

	case muint32:
		if l < 5 {
			return 0, b, ErrShortBytes
		}
		return uint64(getMuint32(b)), b[5:], nil

	case muint64:
		if l < 9 {
			return 0, b, ErrShortBytes
		}
		return getMuint64(b), b[9:], nil

	default:
		return 0, b, badPrefix(UintType, lead)
	}

}

// ReadUint32Bytes tries to read a uint32 from b and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a uint)
// - UintOverflow{} (value too large for uint32)
func ReadUint32Bytes(b []byte) (uint32, []byte, error) {
	v, o, err := ReadUint64Bytes(b)
	if v > math.MaxUint32 {
		return 0, nil, UintOverflow{Value: v, FailedBitsize: 32}
	}
	return uint32(v), o, err
}

// ReadUint16Bytes tries to read a uint16 from b and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a uint)
// - UintOverflow{} (value too large for uint16)
func ReadUint16Bytes(b []byte) (uint16, []byte, error) {
	v, o, err := ReadUint64Bytes(b)
	if v > math.MaxUint16 {
		return 0, nil, UintOverflow{Value: v, FailedBitsize: 16}
	}
	return uint16(v), o, err
}

// ReadUint8Bytes tries to read a uint8 from b and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a uint)
// - UintOverflow{} (value too large for uint8)
func ReadUint8Bytes(b []byte) (uint8, []byte, error) {
	v, o, err := ReadUint64Bytes(b)
	if v > math.MaxUint8 {
		return 0, nil, UintOverflow{Value: v, FailedBitsize: 8}
	}
	return uint8(v), o, err
}

// ReadUintBytes tries to read a uint from b and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a uint)
// - UintOverflow{} (value too large for uint; 32-bit platforms only)
func ReadUintBytes(b []byte) (uint, []byte, error) {
	if smallint {
		u, b, err := ReadUint32Bytes(b)
		return uint(u), b, err
	}
	u, b, err := ReadUint64Bytes(b)
	return uint(u), b, err
}

// ReadByteBytes is analogous to ReadUint8Bytes
func ReadByteBytes(b []byte) (byte, []byte, error) {
	return ReadUint8Bytes(b)
}

// ReadBytesBytes reads a 'bin' object from b and returns its value and any remaining bytes.
// The data is copied to the scratch slice if it's big enough, otherwise a slice is allocated.
// Possible errors are ErrShortBytes and TypeError.
func ReadBytesBytes(b []byte, scratch []byte) ([]byte, []byte, error) {
	return readBytesBytes(b, scratch, false)
}

func readBytesBytes(b []byte, scratch []byte, zc bool) ([]byte, []byte, error) {
	l := len(b)
	if l < 1 {
		return nil, b, ErrShortBytes
	}

	var dataLen int

	switch lead := b[0]; lead {
	case mbin8:
		if l < 2 {
			return nil, b, ErrShortBytes
		}
		dataLen = int(b[1])
		b = b[2:]
	case mbin16:
		if l < 3 {
			return nil, b, ErrShortBytes
		}
		dataLen = int(big.Uint16(b[1:]))
		b = b[3:]
	case mbin32:
		if l < 5 {
			return nil, b, ErrShortBytes
		}
		dataLen = int(big.Uint32(b[1:]))
		b = b[5:]
	default:
		return nil, b, badPrefix(BinType, lead)
	}

	if len(b) < dataLen {
		return nil, b, ErrShortBytes
	}

	// zero-copy
	if zc {
		return b[0:dataLen], b[dataLen:], nil
	}

	if cap(scratch) >= dataLen {
		scratch = scratch[0:dataLen]
	} else {
		scratch = make([]byte, dataLen)
	}

	copy(scratch, b)
	return scratch, b[dataLen:], nil
}

// ReadBytesZC extracts a 'bin' object from b without copying. The first slice returned points
// to the same memory as the input slice, and the second slice is any remaining bytes.
// Possible errors are ErrShortBytes and TypeError.
func ReadBytesZC(b []byte) ([]byte, []byte, error) {
	return readBytesBytes(b, nil, true)
}

// ReadExactBytes reads into dst the bytes expected with the next object in b.
func ReadExactBytes(b []byte, dst []byte) ([]byte, error) {

	l := len(b)
	if l < 1 {
		return b, ErrShortBytes
	}

	var read uint32 // The number of bytes of the data to read.
	var skip int    // The length of the prefix indicating the length of data.

	switch lead := b[0]; lead {
	case mbin8:
		if l < 2 {
			return b, ErrShortBytes
		}
		read = uint32(b[1])
		skip = 2
	case mbin16:
		if l < 3 {
			return b, ErrShortBytes
		}
		read = uint32(big.Uint16(b[1:]))
		skip = 3
	case mbin32:
		if l < 5 {
			return b, ErrShortBytes
		}
		read = big.Uint32(b[1:])
		skip = 5
	default:
		return b, badPrefix(BinType, lead)
	}

	if read != uint32(len(dst)) {
		return b, ArrayError{Wanted: uint32(len(dst)), Got: read}
	}

	return b[skip+copy(dst, b[skip:]):], nil

}

// ReadStringZC reads a MessagePack string field without copying. The returned []byte points
// to the same memory as the input slice. Possible errors are ErrShortBytes (b not long enough)
// and TypeError{} (object not 'str').
func ReadStringZC(b []byte) ([]byte, []byte, error) {

	l := len(b)
	if l < 1 {
		return nil, b, ErrShortBytes
	}

	lead := b[0]
	var read int

	if isfixstr(lead) {
		read = int(rfixstr(lead))
		b = b[1:]
	} else {
		switch lead {
		case mstr8:
			if l < 2 {
				return nil, b, ErrShortBytes
			}
			read = int(b[1])
			b = b[2:]

		case mstr16:
			if l < 3 {
				return nil, b, ErrShortBytes
			}
			read = int(big.Uint16(b[1:]))
			b = b[3:]

		case mstr32:
			if l < 5 {
				return nil, b, ErrShortBytes
			}
			read = int(big.Uint32(b[1:]))
			b = b[5:]

		default:
			return nil, b, TypeError{Method: StrType, Encoded: getType(lead)}
		}
	}

	if len(b) < read {
		return nil, b, ErrShortBytes
	}

	return b[0:read], b[read:], nil

}

// ReadStringBytes reads a 'str' object from b and returns its value and any remaining bytes in b.
// Possible errors are ErrShortBytes, TypeError, and InvalidPrefixError.
func ReadStringBytes(b []byte) (string, []byte, error) {
	v, o, err := ReadStringZC(b)
	return string(v), o, err
}

// ReadStringAsBytes reads a 'str' object into a slice of bytes. The data read is the first slice returned,
// which may be written to the memory held by the scratch slice if it is large enough (scratch may be nil).
// The second slice returned contains the remaining bytes in b. Possible errors are ErrShortBytes (b not
// long enough), TypeError{} (not 'str' type), and InvalidPrefixError (unknown type marker).
func ReadStringAsBytes(b []byte, scratch []byte) ([]byte, []byte, error) {
	tmp, o, err := ReadStringZC(b)
	return append(scratch[:0], tmp...), o, err
}

// ReadComplex128Bytes reads a complex128 extension object from 'b' and returns any remaining bytes.
// Possible errors are ErrShortBytes, TypeError, InvalidPrefixError, and ExtensionTypeError.
func ReadComplex128Bytes(b []byte) (c complex128, o []byte, err error) {
	if len(b) < 18 {
		err = ErrShortBytes
		return
	}
	if b[0] != mfixext16 {
		err = badPrefix(Complex128Type, b[0])
		return
	}
	if int8(b[1]) != Complex128Extension {
		err = errExt(int8(b[1]), Complex128Extension)
		return
	}
	c = complex(math.Float64frombits(big.Uint64(b[2:])),
		math.Float64frombits(big.Uint64(b[10:])))
	o = b[18:]
	return
}

// ReadComplex64Bytes reads a complex64 extension object from b and returns any remaining bytes.
// Possible errors include ErrShortBytes (not enough bytes in slice b), TypeError{} (object not a
// complex64), and ExtensionTypeError{} (object an extension of the correct size, but not a complex64)
func ReadComplex64Bytes(b []byte) (c complex64, o []byte, err error) {
	if len(b) < 10 {
		err = ErrShortBytes
		return
	}
	if b[0] != mfixext8 {
		err = badPrefix(Complex64Type, b[0])
		return
	}
	if b[1] != Complex64Extension {
		err = errExt(int8(b[1]), Complex64Extension)
		return
	}
	c = complex(math.Float32frombits(big.Uint32(b[2:])),
		math.Float32frombits(big.Uint32(b[6:])))
	o = b[10:]
	return
}

// ReadTimeBytes reads a time.Time extension object from b and returns any remaining bytes.
// Possible errors include ErrShortBytes (not enough bytes in b), TypeError{} (object not a time),
// and ExtensionTypeError{} (object an extension of the correct size, but not a time.Time).
func ReadTimeBytes(b []byte) (time.Time, []byte, error) {
	if len(b) < 15 {
		return time.Time{}, b, ErrShortBytes
	}
	if b[0] != mext8 || b[1] != 12 {
		return time.Time{}, b, badPrefix(TimeType, b[0])
	}
	if int8(b[2]) != TimeExtension {
		return time.Time{}, b, errExt(int8(b[2]), TimeExtension)
	}
	sec, nsec := getUnix(b[3:])
	return time.Unix(sec, int64(nsec)).Local(), b[15:], nil
}

// ReadMapStrIntfBytes reads a map[string]interface{} out of b and returns the map and any remaining bytes.
// If map old is not nil, it will be cleared and used so that a map does not need to be created.
func ReadMapStrIntfBytes(b []byte, old map[string]interface{}) (map[string]interface{}, []byte, error) {

	sz, o, err := ReadMapHeaderBytes(b)
	if err != nil {
		return old, o, err
	}

	if old != nil {
		for key := range old {
			delete(old, key)
		}
	} else {
		old = make(map[string]interface{}, int(sz))
	}

	for z := uint32(0); z < sz; z++ {
		if len(o) < 1 {
			return old, o, ErrShortBytes
		}
		var key []byte
		key, o, err = ReadMapKeyZC(o)
		if err != nil {
			return old, o, err
		}
		var val interface{}
		val, o, err = ReadIntfBytes(o)
		if err != nil {
			return old, o, err
		}
		old[string(key)] = val
	}

	return old, o, err

}

// ReadIntfBytes reads the next object out of b as a raw interface{} and returns any remaining bytes.
func ReadIntfBytes(b []byte) (interface{}, []byte, error) {

	if len(b) < 1 {
		return nil, b, ErrShortBytes
	}

	k := NextType(b)

	switch k {
	case MapType:
		return ReadMapStrIntfBytes(b, nil)
	case ArrayType:
		sz, o, err := ReadArrayHeaderBytes(b)
		if err != nil {
			return nil, o, err
		}
		i := make([]interface{}, int(sz))
		for d := range i {
			i[d], o, err = ReadIntfBytes(o)
			if err != nil {
				return i, o, err
			}
		}
		return i, o, nil
	case Float32Type:
		return ReadFloat32Bytes(b)
	case Float64Type:
		return ReadFloat64Bytes(b)
	case IntType:
		return ReadInt64Bytes(b)
	case UintType:
		return ReadUint64Bytes(b)
	case BoolType:
		return ReadBoolBytes(b)
	case TimeType:
		return ReadTimeBytes(b)
	case Complex64Type:
		return ReadComplex64Bytes(b)
	case Complex128Type:
		return ReadComplex128Bytes(b)
	case ExtensionType:
		t, err := peekExtension(b)
		if err != nil {
			return nil, b, err
		}
		// Use a user-defined extension if it's been registered.
		f, ok := extensionReg[t]
		if ok {
			e := f()
			o, err := ReadExtensionBytes(b, e)
			return e, o, err
		}
		// Last resort is a raw extension.
		e := RawExtension{}
		e.Type = int8(t)
		o, err := ReadExtensionBytes(b, &e)
		return &e, o, err
	case NilType:
		o, err := ReadNilBytes(b)
		return nil, o, err
	case BinType:
		return ReadBytesBytes(b, nil)
	case StrType:
		return ReadStringBytes(b)
	default:
		return nil, b[1:], InvalidPrefixError(b[0])
	}

}

// Skip skips the next object in slice b and returns the remaining bytes. If the object
// is a map or array, all of its elements will be skipped. Possible errors are
// ErrShortBytes (not enough bytes in b) and InvalidPrefixError (bad encoding).
func Skip(b []byte) ([]byte, error) {
	sz, asz, err := getSize(b)
	if err != nil {
		return b, err
	}
	if uintptr(len(b)) < sz {
		return b, ErrShortBytes
	}
	b = b[sz:]
	for asz > 0 {
		b, err = Skip(b)
		if err != nil {
			return b, err
		}
		asz--
	}
	return b, nil
}

// getSize returns (skip N bytes, skip M objects, error)
func getSize(b []byte) (uintptr, uintptr, error) {
	l := len(b)
	if l == 0 {
		return 0, 0, ErrShortBytes
	}
	lead := b[0]
	spec := &sizes[lead] // get type information
	size, mode := spec.size, spec.extra
	if size == 0 {
		return 0, 0, InvalidPrefixError(lead)
	}
	if mode >= 0 { // fixed composites
		return uintptr(size), uintptr(mode), nil
	}
	if l < int(size) {
		return 0, 0, ErrShortBytes
	}
	switch mode {
	case extra8:
		return uintptr(size) + uintptr(b[1]), 0, nil
	case extra16:
		return uintptr(size) + uintptr(big.Uint16(b[1:])), 0, nil
	case extra32:
		return uintptr(size) + uintptr(big.Uint32(b[1:])), 0, nil
	case map16v:
		return uintptr(size), 2 * uintptr(big.Uint16(b[1:])), nil
	case map32v:
		return uintptr(size), 2 * uintptr(big.Uint32(b[1:])), nil
	case array16v:
		return uintptr(size), uintptr(big.Uint16(b[1:])), nil
	case array32v:
		return uintptr(size), uintptr(big.Uint32(b[1:])), nil
	default:
		return 0, 0, fatal
	}
}
