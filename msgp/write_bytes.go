package msgp

import (
	"math"
	"reflect"
	"time"
)

// Require returns a slice (with the contents of old) having a capacity to fit at
// least extra number of bytes after the current length. The length of the returned
// slice is the same as the length of the old slice.
func Require(old []byte, extra int) []byte {
	l := len(old)
	c := cap(old)
	r := l + extra
	if c >= r {
		return old
	}
	// The new size is the greater of double the old capacity and
	// the sum of the old length and the number of new bytes needed.
	c <<= 1
	if c < r {
		c = r
	}
	n := make([]byte, l, c)
	copy(n, old)
	return n
}

// ensure returns a slice (with the contents of b) having at least size extra bytes between
// its length and capacity. The int returned indicates the index at which to write.
func ensure(b []byte, size int) ([]byte, int) {
	b = Require(b, size)
	l := len(b)
	return b[:l+size], l
}

// AppendMapHeader appends a map header of the given size (number of elements) to b.
func AppendMapHeader(b []byte, size uint32) []byte {
	if size <= 15 {
		return append(b, wfixmap(uint8(size)))
	}
	if size <= math.MaxUint16 {
		o, n := ensure(b, 3)
		prefixu16(o[n:], mmap16, uint16(size))
		return o
	}
	o, n := ensure(b, 5)
	prefixu32(o[n:], mmap32, size)
	return o
}

// AppendArrayHeader appends an array header of the given size to b.
func AppendArrayHeader(b []byte, size uint32) []byte {
	if size <= 15 {
		return append(b, wfixarray(uint8(size)))
	}
	if size <= math.MaxUint16 {
		o, n := ensure(b, 3)
		prefixu16(o[n:], marray16, uint16(size))
		return o
	}
	o, n := ensure(b, 5)
	prefixu32(o[n:], marray32, size)
	return o
}

// AppendNil appends a MessagePack nil byte to b.
func AppendNil(b []byte) []byte { return append(b, mnil) }

// AppendFloat64 appends a float64 to b.
func AppendFloat64(b []byte, f float64) []byte {
	o, n := ensure(b, Float64Size)
	prefixu64(o[n:], mfloat64, math.Float64bits(f))
	return o
}

// AppendFloat32 appends a float32 to b.
func AppendFloat32(b []byte, f float32) []byte {
	o, n := ensure(b, Float32Size)
	prefixu32(o[n:], mfloat32, math.Float32bits(f))
	return o
}

// AppendInt64 appends an int64 to b.
func AppendInt64(b []byte, i int64) []byte {
	if i >= 0 {
		switch {
		case i <= math.MaxInt8:
			return append(b, wfixint(uint8(i)))
		case i <= math.MaxInt16:
			o, n := ensure(b, 3)
			putMint16(o[n:], int16(i))
			return o
		case i <= math.MaxInt32:
			o, n := ensure(b, 5)
			putMint32(o[n:], int32(i))
			return o
		default:
			o, n := ensure(b, 9)
			putMint64(o[n:], i)
			return o
		}
	}
	switch {
	case i >= -32:
		return append(b, wnfixint(int8(i)))
	case i >= math.MinInt8:
		o, n := ensure(b, 2)
		putMint8(o[n:], int8(i))
		return o
	case i >= math.MinInt16:
		o, n := ensure(b, 3)
		putMint16(o[n:], int16(i))
		return o
	case i >= math.MinInt32:
		o, n := ensure(b, 5)
		putMint32(o[n:], int32(i))
		return o
	default:
		o, n := ensure(b, 9)
		putMint64(o[n:], i)
		return o
	}
}

// AppendInt8 appends an int8 to b.
func AppendInt8(b []byte, i int8) []byte {
	if i >= 0 {
		return append(b, wfixint(uint8(i)))
	}
	if i >= -32 {
		return append(b, wnfixint(i))
	}
	o, n := ensure(b, 2)
	putMint8(o[n:], i)
	return o
}

// AppendInt16 appends an int16 to b.
func AppendInt16(b []byte, i int16) []byte { return AppendInt64(b, int64(i)) }

// AppendInt32 appends an int32 to b.
func AppendInt32(b []byte, i int32) []byte { return AppendInt64(b, int64(i)) }

// AppendUint64 appends a uint64 to b.
func AppendUint64(b []byte, u uint64) []byte {
	switch {
	case u <= math.MaxInt8:
		return append(b, wfixint(uint8(u)))
	case u <= math.MaxUint8:
		o, n := ensure(b, 2)
		putMuint8(o[n:], uint8(u))
		return o
	case u <= math.MaxUint16:
		o, n := ensure(b, 3)
		putMuint16(o[n:], uint16(u))
		return o
	case u <= math.MaxUint32:
		o, n := ensure(b, 5)
		putMuint32(o[n:], uint32(u))
		return o
	default:
		o, n := ensure(b, 9)
		putMuint64(o[n:], u)
		return o
	}
}

// AppendUint appends a uint to b.
func AppendUint(b []byte, u uint) []byte { return AppendUint64(b, uint64(u)) }

// AppendUint8 appends a uint8 to b.
func AppendUint8(b []byte, u uint8) []byte {
	if u <= math.MaxInt8 {
		return append(b, wfixint(u))
	}
	o, n := ensure(b, 2)
	putMuint8(o[n:], u)
	return o
}

// AppendUint16 appends a uint16 to b.
func AppendUint16(b []byte, u uint16) []byte { return AppendUint64(b, uint64(u)) }

// AppendUint32 appends a uint32 to b.
func AppendUint32(b []byte, u uint32) []byte { return AppendUint64(b, uint64(u)) }

// AppendInt appends an int to b.
func AppendInt(b []byte, i int) []byte { return AppendInt64(b, int64(i)) }

// AppendByte does the same thing as AppendUint8.
func AppendByte(b []byte, u byte) []byte { return AppendUint8(b, u) }

// AppendBytes appends slice bts to b as MessagePack 'bin' data.
func AppendBytes(b []byte, bts []byte) []byte {
	sz := len(bts)
	var n int
	if sz <= math.MaxUint8 {
		b, n = ensure(b, 2+sz)
		prefixu8(b[n:], mbin8, uint8(sz))
		n += 2
	} else if sz <= math.MaxUint16 {
		b, n = ensure(b, 3+sz)
		prefixu16(b[n:], mbin16, uint16(sz))
		n += 3
	} else {
		b, n = ensure(b, 5+sz)
		prefixu32(b[n:], mbin32, uint32(sz))
		n += 5
	}
	return b[:n+copy(b[n:], bts)]
}

// AppendBool appends a bool b.
func AppendBool(b []byte, t bool) []byte {
	if t {
		return append(b, mtrue)
	}
	return append(b, mfalse)
}

// AppendString appends a string as a MessagePack 'str' to b.
func AppendString(b []byte, s string) []byte {
	sz := len(s)
	var n int
	switch {
	case sz <= 31:
		b, n = ensure(b, 1+sz)
		b[n] = wfixstr(uint8(sz))
		n++
	case sz <= math.MaxUint8:
		b, n = ensure(b, 2+sz)
		prefixu8(b[n:], mstr8, uint8(sz))
		n += 2
	case sz <= math.MaxUint16:
		b, n = ensure(b, 3+sz)
		prefixu16(b[n:], mstr16, uint16(sz))
		n += 3
	default:
		b, n = ensure(b, 5+sz)
		prefixu32(b[n:], mstr32, uint32(sz))
		n += 5
	}
	return b[:n+copy(b[n:], s)]
}

// AppendComplex64 appends a complex64 to b as a MessagePack extension.
func AppendComplex64(b []byte, c complex64) []byte {
	o, n := ensure(b, Complex64Size)
	o[n] = mfixext8
	o[n+1] = Complex64Extension
	big.PutUint32(o[n+2:], math.Float32bits(real(c)))
	big.PutUint32(o[n+6:], math.Float32bits(imag(c)))
	return o
}

// AppendComplex128 appends a complex128 to b as a MessagePack extension.
func AppendComplex128(b []byte, c complex128) []byte {
	o, n := ensure(b, Complex128Size)
	o[n] = mfixext16
	o[n+1] = Complex128Extension
	big.PutUint64(o[n+2:], math.Float64bits(real(c)))
	big.PutUint64(o[n+10:], math.Float64bits(imag(c)))
	return o
}

// AppendTime appends a time.Time to the slice as a MessagePack extension
func AppendTime(b []byte, t time.Time) []byte {
	o, n := ensure(b, TimeSize)
	t = t.UTC()
	o[n] = mext8
	o[n+1] = 12
	o[n+2] = TimeExtension
	putUnix(o[n+3:], t.Unix(), int32(t.Nanosecond()))
	return o
}

// AppendMapStrStr appends a map[string]string to b as a MessagePack map
// with 'str'-type keys and values.
func AppendMapStrStr(b []byte, m map[string]string) []byte {
	sz := uint32(len(m))
	b = AppendMapHeader(b, sz)
	for key, val := range m {
		b = AppendString(b, key)
		b = AppendString(b, val)
	}
	return b
}

// AppendMapStrIntf appends a map[string]interface{} to b as a MessagePack map
// with 'str'-type keys.
func AppendMapStrIntf(b []byte, m map[string]interface{}) ([]byte, error) {
	sz := uint32(len(m))
	b = AppendMapHeader(b, sz)
	var err error
	for key, val := range m {
		b = AppendString(b, key)
		b, err = AppendIntf(b, val)
		if err != nil {
			return b, err
		}
	}
	return b, nil
}

// AppendIntf appends the concrete type of i to b. The type of i must be
// one of the following:
//  - bool, float, string, []byte, int, uint, complex, time.Time, or nil
//  - map[string]interface{} or map[string]string
//  - []T, where T is another supported type
//  - *T, where T is another supported type
//  - type that implements the msgp.Marshaler interface
//  - type that implements the msgp.Extension interface
func AppendIntf(b []byte, i interface{}) ([]byte, error) {

	if i == nil {
		return AppendNil(b), nil
	}

	// Here are all the concrete types for which we have methods.
	switch i := i.(type) {
	case Marshaler:
		return i.MarshalMsg(b)
	case Extension:
		return AppendExtension(b, i)
	case bool:
		return AppendBool(b, i), nil
	case float32:
		return AppendFloat32(b, i), nil
	case float64:
		return AppendFloat64(b, i), nil
	case complex64:
		return AppendComplex64(b, i), nil
	case complex128:
		return AppendComplex128(b, i), nil
	case string:
		return AppendString(b, i), nil
	case []byte:
		return AppendBytes(b, i), nil
	case int8:
		return AppendInt8(b, i), nil
	case int16:
		return AppendInt16(b, i), nil
	case int32:
		return AppendInt32(b, i), nil
	case int64:
		return AppendInt64(b, i), nil
	case int:
		return AppendInt64(b, int64(i)), nil
	case uint:
		return AppendUint64(b, uint64(i)), nil
	case uint8:
		return AppendUint8(b, i), nil
	case uint16:
		return AppendUint16(b, i), nil
	case uint32:
		return AppendUint32(b, i), nil
	case uint64:
		return AppendUint64(b, i), nil
	case time.Time:
		return AppendTime(b, i), nil
	case map[string]interface{}:
		return AppendMapStrIntf(b, i)
	case map[string]string:
		return AppendMapStrStr(b, i), nil
	case []interface{}:
		b = AppendArrayHeader(b, uint32(len(i)))
		var err error
		for _, k := range i {
			b, err = AppendIntf(b, k)
			if err != nil {
				return b, err
			}
		}
		return b, nil
	}

	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		l := v.Len()
		b = AppendArrayHeader(b, uint32(l))
		var err error
		for i := 0; i < l; i++ {
			b, err = AppendIntf(b, v.Index(i).Interface())
			if err != nil {
				return b, err
			}
		}
		return b, nil
	case reflect.Ptr:
		if v.IsNil() {
			return AppendNil(b), nil
		}
		return AppendIntf(b, v.Elem().Interface())
	default:
		return b, &ErrUnsupportedType{T: v.Type()}
	}

}
