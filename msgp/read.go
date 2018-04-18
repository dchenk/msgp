package msgp

import (
	"io"
	"math"
	"time"

	"github.com/philhofer/fwd"
)

// smallint says if int and uint types are 32 bits.
const smallint = (32 << (^uint(0) >> 63)) == 32

// A Type is a MessagePack wire type, including this
// package's built-in extension types.
type Type byte

const (
	InvalidType Type = iota // InvalidType is the zero value of Type (not used).

	// MessagePack built-in types
	StrType
	BinType
	MapType
	ArrayType
	Float64Type
	Float32Type
	BoolType
	IntType
	UintType
	NilType
	ExtensionType

	// Types provided by extensions
	Complex64Type
	Complex128Type
	TimeType

	_maxtype
)

// String implements fmt.Stringer
func (t Type) String() string {
	switch t {
	case StrType:
		return "str"
	case BinType:
		return "bin"
	case MapType:
		return "map"
	case ArrayType:
		return "array"
	case Float64Type:
		return "float64"
	case Float32Type:
		return "float32"
	case BoolType:
		return "bool"
	case UintType:
		return "uint"
	case IntType:
		return "int"
	case ExtensionType:
		return "ext"
	case NilType:
		return "nil"
	default:
		return "<invalid>"
	}
}

// Unmarshaler is the interface implemented by objects that know how to unmarshal themselves from
// MessagePack. UnmarshalMsg unmarshals the object from binary, returning any leftover bytes and
// any errors encountered.
type Unmarshaler interface {
	UnmarshalMsg([]byte) ([]byte, error)
}

// Decoder is the interface implemented by objects that know how to read themselves from a *Reader.
type Decoder interface {
	DecodeMsg(*Reader) error
}

// Decode decodes d from r.
func Decode(r io.Reader, d Decoder) error {
	rd := NewReader(r)
	return d.DecodeMsg(rd)
}

// NewReader returns a *Reader that reads from the provided reader. The reader will be buffered.
func NewReader(r io.Reader) *Reader {
	return &Reader{R: fwd.NewReader(r)}
}

// NewReaderSize returns a *Reader with a buffer of the given size. (This is vastly preferable
// to passing the decoder a reader that is already buffered.)
func NewReaderSize(r io.Reader, sz int) *Reader {
	return &Reader{R: fwd.NewReaderSize(r, sz)}
}

// Reader wraps an io.Reader and provides methods to read MessagePack-encoded values from it.
// Readers are buffered.
type Reader struct {
	// R is the buffered reader that the Reader uses to decode MessagePack.
	// The Reader itself is stateless; all the buffering is done within R.
	R       *fwd.Reader
	scratch []byte
}

// Read implements io.Reader.
func (m *Reader) Read(p []byte) (int, error) {
	return m.R.Read(p)
}

// CopyNext reads the next object from m without decoding it and writes it to w.
// It avoids unnecessary copies internally.
func (m *Reader) CopyNext(w io.Writer) (int64, error) {
	sz, o, err := getNextSize(m.R)
	if err != nil {
		return 0, err
	}

	var n int64
	// Opportunistic optimization: if we can fit the whole thing in the m.R buffer,
	// then just get a pointer to that and pass it to w.Write, avoiding an allocation.
	if int(sz) <= m.R.BufferSize() {
		var nn int
		var buf []byte
		buf, err = m.R.Next(int(sz))
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				err = ErrShortBytes
			}
			return 0, err
		}
		nn, err = w.Write(buf)
		n += int64(nn)
	} else {
		// Fall back to io.CopyN.
		// May avoid allocating if w is a ReaderFrom (e.g. bytes.Buffer)
		n, err = io.CopyN(w, m.R, int64(sz))
		if err == io.ErrUnexpectedEOF {
			err = ErrShortBytes
		}
	}
	if err != nil {
		return n, err
	} else if n < int64(sz) {
		return n, io.ErrShortWrite
	}

	// For maps and slices, read elements
	for x := uintptr(0); x < o; x++ {
		n2, err := m.CopyNext(w)
		if err != nil {
			return n, err
		}
		n += n2
	}
	return n, nil
}

// ReadFull implements io.ReadFull.
func (m *Reader) ReadFull(p []byte) (int, error) {
	return m.R.ReadFull(p)
}

// Reset resets the underlying reader.
func (m *Reader) Reset(r io.Reader) { m.R.Reset(r) }

// Buffered returns the number of bytes currently in the read buffer.
func (m *Reader) Buffered() int { return m.R.Buffered() }

// BufferSize returns the capacity of the read buffer.
func (m *Reader) BufferSize() int { return m.R.BufferSize() }

// NextType returns the next object type to be decoded.
func (m *Reader) NextType() (Type, error) {
	p, err := m.R.Peek(1)
	if err != nil {
		return InvalidType, err
	}
	t := getType(p[0])
	if t == InvalidType {
		return t, InvalidPrefixError(p[0])
	}
	if t == ExtensionType {
		v, err := m.peekExtensionType()
		if err != nil {
			return InvalidType, err
		}
		switch v {
		case Complex64Extension:
			return Complex64Type, nil
		case Complex128Extension:
			return Complex128Type, nil
		case TimeExtension:
			return TimeType, nil
		}
	}
	return t, nil
}

// IsNil says whether or not the next byte is a nil MessagePack byte (0xc0).
func (m *Reader) IsNil() bool {
	p, err := m.R.Peek(1)
	return err == nil && p[0] == mnil
}

// getNextSize returns the size of the next object on the wire.
// returns (obj size, obj elements, error) only maps and arrays have non-zero obj elements.
// For maps and arrays, obj size does not include elements.
//
// Use uintptr because it will be large enough to hold whatever we can fit in memory.
func getNextSize(r *fwd.Reader) (uintptr, uintptr, error) {
	b, err := r.Peek(1)
	if err != nil {
		return 0, 0, err
	}
	lead := b[0]
	spec := &sizes[lead]
	size, mode := spec.size, spec.extra
	if size == 0 {
		return 0, 0, InvalidPrefixError(lead)
	}
	if mode >= 0 {
		return uintptr(size), uintptr(mode), nil
	}
	b, err = r.Peek(int(size))
	if err != nil {
		return 0, 0, err
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

// Skip skips over the next object, regardless of its type. If it is an array
// or map, the whole array or map will be skipped.
func (m *Reader) Skip() error {

	var v, o uintptr // v is number of bytes, o is number of objects

	// It's faster to use buffered data if there is enough of it.
	if m.R.Buffered() >= 5 {
		p, err := m.R.Peek(5)
		if err != nil {
			return err
		}
		v, o, err = getSize(p)
		if err != nil {
			return err
		}
	} else {
		var err error
		v, o, err = getNextSize(m.R)
		if err != nil {
			return err
		}
	}

	// v is always non-zero if err == nil
	_, err := m.R.Skip(int(v))
	if err != nil {
		return err
	}

	// for maps and slices, skip elements
	for x := uintptr(0); x < o; x++ {
		err = m.Skip()
		if err != nil {
			return err
		}
	}

	return nil

}

// ReadMapHeader reads the next object as a map header and returns the size of the map.
// A TypeError{} is returned if the next object is not a map.
func (m *Reader) ReadMapHeader() (uint32, error) {
	p, err := m.R.Peek(1)
	if err != nil {
		return 0, err
	}
	lead := p[0]
	if isfixmap(lead) {
		_, err = m.R.Skip(1)
		return uint32(rfixmap(lead)), err
	}
	switch lead {
	case mmap16:
		p, err = m.R.Next(3)
		if err != nil {
			return 0, err
		}
		return uint32(big.Uint16(p[1:])), nil
	case mmap32:
		p, err = m.R.Next(5)
		if err != nil {
			return 0, err
		}
		return big.Uint32(p[1:]), nil
	default:
		return 0, badPrefix(MapType, lead)
	}
}

// ReadMapKey reads either a 'str' or 'bin' field from the reader and returns the value as a []byte.
// It uses scratch for storage if it is large enough.
func (m *Reader) ReadMapKey(scratch []byte) ([]byte, error) {
	out, err := m.ReadStringAsBytes(scratch)
	if err != nil {
		if tperr, ok := err.(TypeError); ok && tperr.Encoded == BinType {
			return m.ReadBytes(scratch)
		}
		return nil, err
	}
	return out, nil
}

// ReadMapKeyPtr returns a []byte pointing to the contents of a valid map key.
// The key cannot be empty, and it must be shorter than the total buffer size of the *Reader.
// The returned slice is only valid until the next *Reader method call. Be extremely careful
// when using this method; writing into the returned slice may corrupt future reads.
func (m *Reader) ReadMapKeyPtr() ([]byte, error) {
	p, err := m.R.Peek(1)
	if err != nil {
		return nil, err
	}
	lead := p[0]
	var read int
	if isfixstr(lead) {
		read = int(rfixstr(lead))
		m.R.Skip(1)
	} else {
		switch lead {
		case mstr8, mbin8:
			p, err = m.R.Next(2)
			if err != nil {
				return nil, err
			}
			read = int(p[1])
		case mstr16, mbin16:
			p, err = m.R.Next(3)
			if err != nil {
				return nil, err
			}
			read = int(big.Uint16(p[1:]))
		case mstr32, mbin32:
			p, err = m.R.Next(5)
			if err != nil {
				return nil, err
			}
			read = int(big.Uint32(p[1:]))
		default:
			return nil, badPrefix(StrType, lead)
		}
	}
	if read == 0 {
		return nil, ErrShortBytes
	}
	return m.R.Next(read)
}

// ReadArrayHeader reads the next object as an array header and returns the size of the array.
func (m *Reader) ReadArrayHeader() (uint32, error) {
	p, err := m.R.Peek(1)
	if err != nil {
		return 0, err
	}
	lead := p[0]
	if isfixarray(lead) {
		_, err = m.R.Skip(1)
		return uint32(rfixarray(lead)), err
	}
	switch lead {
	case marray16:
		p, err = m.R.Next(3)
		if err != nil {
			return 0, err
		}
		return uint32(big.Uint16(p[1:])), nil
	case marray32:
		p, err = m.R.Next(5)
		if err != nil {
			return 0, err
		}
		return big.Uint32(p[1:]), nil
	default:
		return 0, badPrefix(ArrayType, lead)
	}
}

// ReadNil reads a 'nil' MessagePack byte from the reader.
func (m *Reader) ReadNil() error {
	p, err := m.R.Peek(1)
	if err != nil {
		return err
	}
	if p[0] != mnil {
		return badPrefix(NilType, p[0])
	}
	_, err = m.R.Skip(1)
	return err
}

// ReadFloat64 reads a float64 from the reader. If the value on the wire is encoded as a float32,
// it will be converted to a float64 without losing precision.
func (m *Reader) ReadFloat64() (float64, error) {
	p, err := m.R.Peek(9)
	if err != nil {
		if err == io.EOF && len(p) > 0 && p[0] == mfloat32 {
			ef, err := m.ReadFloat32()
			return float64(ef), err
		}
		return 0, err
	}
	if p[0] != mfloat64 {
		if p[0] == mfloat32 {
			ef, err := m.ReadFloat32()
			return float64(ef), err
		}
		return 0, badPrefix(Float64Type, p[0])
	}
	_, err = m.R.Skip(9)
	return math.Float64frombits(getMuint64(p)), err
}

// ReadFloat32 reads a float32 from the reader.
func (m *Reader) ReadFloat32() (float32, error) {
	p, err := m.R.Peek(5)
	if err != nil {
		return 0, err
	}
	if p[0] != mfloat32 {
		return 0, badPrefix(Float32Type, p[0])
	}
	_, err = m.R.Skip(5)
	return math.Float32frombits(getMuint32(p)), err
}

// ReadBool reads a bool from the reader.
func (m *Reader) ReadBool() (bool, error) {
	p, err := m.R.Peek(1)
	if err != nil {
		return false, err
	}
	if p[0] != mtrue && p[0] != mfalse {
		return false, badPrefix(BoolType, p[0])
	}
	_, err = m.R.Skip(1)
	return p[0] == mtrue, err
}

// ReadInt64 reads an int64 from the reader. If an int64 is not available, this function tries to read
// an unsigned integer and convert it to an int64 if possible. Errors that can be returned include
// UintOverflow and TypeError (for bad prefix).
func (m *Reader) ReadInt64() (int64, error) {

	p, err := m.R.Peek(1)
	if err != nil {
		return 0, err
	}
	lead := p[0]

	if isfixint(lead) {
		_, err = m.R.Skip(1)
		return int64(rfixint(lead)), err
	} else if isnfixint(lead) {
		_, err = m.R.Skip(1)
		return int64(rnfixint(lead)), err
	}

	switch lead {
	case mint8:
		p, err = m.R.Next(2)
		if err != nil {
			return 0, err
		}
		return int64(getMint8(p)), nil
	case mint16:
		p, err = m.R.Next(3)
		if err != nil {
			return 0, err
		}
		return int64(getMint16(p)), nil
	case mint32:
		p, err = m.R.Next(5)
		if err != nil {
			return 0, err
		}
		return int64(getMint32(p)), nil
	case mint64:
		p, err = m.R.Next(9)
		if err != nil {
			return 0, err
		}
		return getMint64(p), nil
	// At this point, we can just try to read an unsigned integer.
	case muint8:
		p, err = m.R.Next(2)
		if err != nil {
			return 0, err
		}
		return int64(getMuint8(p)), nil
	case muint16:
		p, err = m.R.Next(3)
		if err != nil {
			return 0, err
		}
		return int64(getMuint16(p)), nil
	case muint32:
		p, err = m.R.Next(5)
		if err != nil {
			return 0, err
		}
		return int64(getMuint32(p)), nil
	case muint64:
		p, err = m.R.Next(9)
		if err != nil {
			return 0, err
		}
		num := getMuint64(p)
		// Check for overflow only with uint64 because all other (smaller) unsigned
		// integers can fit into an int64.
		if num > math.MaxInt64 {
			return 0, UintOverflow{num, 64}
		}
		return int64(num), nil
	}

	return 0, badPrefix(IntType, lead)

}

// ReadInt32 reads an int32 from the reader
func (m *Reader) ReadInt32() (int32, error) {
	in, err := m.ReadInt64()
	if in > math.MaxInt32 || in < math.MinInt32 {
		return 0, IntOverflow{Value: in, FailedBitsize: 32}
	}
	return int32(in), err
}

// ReadInt16 reads an int16 from the reader.
func (m *Reader) ReadInt16() (int16, error) {
	in, err := m.ReadInt64()
	if in > math.MaxInt16 || in < math.MinInt16 {
		return 0, IntOverflow{Value: in, FailedBitsize: 16}
	}
	return int16(in), err
}

// ReadInt8 reads an int8 from the reader
func (m *Reader) ReadInt8() (int8, error) {
	in, err := m.ReadInt64()
	if in > math.MaxInt8 || in < math.MinInt8 {
		return 0, IntOverflow{Value: in, FailedBitsize: 8}
	}
	return int8(in), err
}

// ReadInt reads an int from the reader
func (m *Reader) ReadInt() (int, error) {
	if smallint {
		in, err := m.ReadInt32()
		return int(in), err
	}
	in, err := m.ReadInt64()
	return int(in), err
}

// ReadUint64 reads a uint64 from the reader.
func (m *Reader) ReadUint64() (uint64, error) {

	p, err := m.R.Peek(1)
	if err != nil {
		return 0, err
	}
	lead := p[0]

	if isfixint(lead) {
		_, err = m.R.Skip(1)
		if err != nil {
			return 0, err
		}
		return uint64(rfixint(lead)), nil
	}

	switch lead {
	case muint8:
		p, err = m.R.Next(2)
		if err != nil {
			return 0, err
		}
		return uint64(getMuint8(p)), nil
	case muint16:
		p, err = m.R.Next(3)
		if err != nil {
			return 0, err
		}
		return uint64(getMuint16(p)), nil
	case muint32:
		p, err = m.R.Next(5)
		if err != nil {
			return 0, err
		}
		return uint64(getMuint32(p)), nil
	case muint64:
		p, err = m.R.Next(9)
		if err != nil {
			return 0, err
		}
		return getMuint64(p), nil
	default:
		return 0, badPrefix(UintType, lead)
	}

}

// ReadUint32 reads a uint32 from the reader
func (m *Reader) ReadUint32() (u uint32, err error) {
	in, err := m.ReadUint64()
	if in > math.MaxUint32 {
		return 0, UintOverflow{Value: in, FailedBitsize: 32}
	}
	return uint32(in), err
}

// ReadUint16 reads a uint16 from the reader
func (m *Reader) ReadUint16() (uint16, error) {
	in, err := m.ReadUint64()
	if in > math.MaxUint16 {
		return 0, UintOverflow{Value: in, FailedBitsize: 16}
	}
	return uint16(in), err
}

// ReadUint8 reads a uint8 from the reader.
func (m *Reader) ReadUint8() (uint8, error) {
	in, err := m.ReadUint64()
	if in > math.MaxUint8 {
		return 0, UintOverflow{in, 8}
	}
	return uint8(in), err
}

// ReadUint reads a uint from the reader
func (m *Reader) ReadUint() (uint, error) {
	if smallint {
		un, err := m.ReadUint32()
		return uint(un), err
	}
	un, err := m.ReadUint64()
	return uint(un), err
}

// ReadByte is analogous to ReadUint8.
// This is *not* an implementation of io.ByteReader.
func (m *Reader) ReadByte() (byte, error) {
	in, err := m.ReadUint64()
	if in > math.MaxUint8 {
		return 0x00, UintOverflow{in, 8}
	}
	return byte(in), err
}

// ReadBytes reads a MessagePack 'bin' object from the reader and returns its value.
// It may use the scratch slice for storage if it is non-nil.
func (m *Reader) ReadBytes(scratch []byte) ([]byte, error) {
	p, err := m.R.Peek(2)
	if err != nil {
		return nil, err
	}
	lead := p[0]
	var dataLen int64
	switch lead {
	case mbin8:
		dataLen = int64(p[1])
		m.R.Skip(2)
	case mbin16:
		p, err = m.R.Next(3)
		if err != nil {
			return nil, err
		}
		dataLen = int64(big.Uint16(p[1:]))
	case mbin32:
		p, err = m.R.Next(5)
		if err != nil {
			return nil, err
		}
		dataLen = int64(big.Uint32(p[1:]))
	default:
		return nil, badPrefix(BinType, lead)
	}
	var b []byte
	if int64(cap(scratch)) < dataLen {
		b = make([]byte, dataLen)
	} else {
		b = scratch[0:dataLen]
	}
	_, err = m.R.ReadFull(b)
	return b, err
}

// ReadBytesHeader reads the size header of a MessagePack 'bin' object. The user is responsible
// for dealing with the next 'sz' bytes from the reader in an application-specific way.
func (m *Reader) ReadBytesHeader() (uint32, error) {
	p, err := m.R.Peek(1)
	if err != nil {
		return 0, err
	}
	switch p[0] {
	case mbin8:
		p, err = m.R.Next(2)
		if err != nil {
			return 0, err
		}
		return uint32(p[1]), nil
	case mbin16:
		p, err = m.R.Next(3)
		if err != nil {
			return 0, err
		}
		return uint32(big.Uint16(p[1:])), nil
	case mbin32:
		p, err = m.R.Next(5)
		if err != nil {
			return 0, err
		}
		return big.Uint32(p[1:]), nil
	default:
		return 0, badPrefix(BinType, p[0])
	}
}

// ReadExactBytes reads a MessagePack 'bin'-encoded object off of the wire into the provided slice.
// An ArrayError will be returned if the object is not exactly the length of the input slice.
func (m *Reader) ReadExactBytes(into []byte) error {
	p, err := m.R.Peek(2)
	if err != nil {
		return err
	}
	var read int64 // bytes to read
	var skip int   // prefix size to skip
	switch lead := p[0]; lead {
	case mbin8:
		read = int64(p[1])
		skip = 2
	case mbin16:
		p, err = m.R.Peek(3)
		if err != nil {
			return err
		}
		read = int64(big.Uint16(p[1:]))
		skip = 3
	case mbin32:
		p, err = m.R.Peek(5)
		if err != nil {
			return err
		}
		read = int64(big.Uint32(p[1:]))
		skip = 5
	default:
		return badPrefix(BinType, lead)
	}
	if read != int64(len(into)) {
		return ArrayError{Wanted: uint32(len(into)), Got: uint32(read)}
	}
	m.R.Skip(skip)
	_, err = m.R.ReadFull(into)
	return err
}

// ReadStringAsBytes reads a MessagePack 'str' (UTF-8) string and returns its value as bytes.
// The scratch slice will be used for storage if it is not nil and large enough.
func (m *Reader) ReadStringAsBytes(scratch []byte) ([]byte, error) {

	p, err := m.R.Peek(1)
	if err != nil {
		return scratch, err
	}

	lead := p[0]
	var read int64

	if isfixstr(lead) {
		read = int64(rfixstr(lead))
		_, err = m.R.Skip(1)
		if err != nil {
			return scratch, err
		}
	} else {
		switch lead {
		case mstr8:
			p, err = m.R.Next(2)
			if err != nil {
				return scratch, err
			}
			read = int64(uint8(p[1]))
		case mstr16:
			p, err = m.R.Next(3)
			if err != nil {
				return scratch, err
			}
			read = int64(big.Uint16(p[1:]))
		case mstr32:
			p, err = m.R.Next(5)
			if err != nil {
				return scratch, err
			}
			read = int64(big.Uint32(p[1:]))
		default:
			return scratch, badPrefix(StrType, lead)
		}
	}

	if int64(cap(scratch)) < read {
		scratch = make([]byte, read)
	} else {
		scratch = scratch[0:read]
	}

	_, err = m.R.ReadFull(scratch)
	return scratch, err

}

// ReadStringHeader reads a string header off of the wire. The user is then responsible
// for dealing with the next sz bytes from the reader in an application-specific manner.
func (m *Reader) ReadStringHeader() (sz uint32, err error) {
	var p []byte
	p, err = m.R.Peek(1)
	if err != nil {
		return
	}
	lead := p[0]
	if isfixstr(lead) {
		sz = uint32(rfixstr(lead))
		m.R.Skip(1)
		return
	}
	switch lead {
	case mstr8:
		p, err = m.R.Next(2)
		if err != nil {
			return
		}
		sz = uint32(p[1])
		return
	case mstr16:
		p, err = m.R.Next(3)
		if err != nil {
			return
		}
		sz = uint32(big.Uint16(p[1:]))
		return
	case mstr32:
		p, err = m.R.Next(5)
		if err != nil {
			return
		}
		sz = big.Uint32(p[1:])
		return
	default:
		err = badPrefix(StrType, lead)
		return
	}
}

// ReadString reads a UTF-8 string from the reader.
func (m *Reader) ReadString() (string, error) {

	p, err := m.R.Peek(1)
	if err != nil {
		return "", err
	}
	lead := p[0]

	var read uint32
	if isfixstr(lead) {
		read = uint32(rfixstr(lead))
		_, err = m.R.Skip(1)
		if err != nil {
			return "", err
		}
	} else {
		switch lead {
		case mstr8:
			p, err = m.R.Next(2)
			if err != nil {
				return "", err
			}
			read = uint32(p[1])
		case mstr16:
			p, err = m.R.Next(3)
			if err != nil {
				return "", err
			}
			read = uint32(big.Uint16(p[1:]))
		case mstr32:
			p, err = m.R.Next(5)
			if err != nil {
				return "", err
			}
			read = big.Uint32(p[1:])
		default:
			return "", badPrefix(StrType, lead)
		}
	}

	out := make([]byte, read)
	_, err = m.R.ReadFull(out)
	return string(out), err

}

// ReadComplex64 reads a complex64 from the reader.
func (m *Reader) ReadComplex64() (complex64, error) {
	p, err := m.R.Peek(10)
	if err != nil {
		return 0, err
	}
	if p[0] != mfixext8 {
		return 0, badPrefix(Complex64Type, p[0])
	}
	if int8(p[1]) != Complex64Extension {
		return 0, errExt(int8(p[1]), Complex64Extension)
	}
	f := complex(math.Float32frombits(big.Uint32(p[2:])),
		math.Float32frombits(big.Uint32(p[6:])))
	_, err = m.R.Skip(10)
	return f, err
}

// ReadComplex128 reads a complex128 from the reader.
func (m *Reader) ReadComplex128() (complex128, error) {
	p, err := m.R.Peek(18)
	if err != nil {
		return 0, err
	}
	if p[0] != mfixext16 {
		return 0, badPrefix(Complex128Type, p[0])
	}
	if int8(p[1]) != Complex128Extension {
		return 0, errExt(int8(p[1]), Complex128Extension)
	}
	f := complex(math.Float64frombits(big.Uint64(p[2:])),
		math.Float64frombits(big.Uint64(p[10:])))
	_, err = m.R.Skip(18)
	return f, err
}

// ReadMapStrIntf reads a MessagePack map into a map[string]interface{}.
// You must pass a non-nil map into the function.
func (m *Reader) ReadMapStrIntf(mp map[string]interface{}) error {
	sz, err := m.ReadMapHeader()
	if err != nil {
		return err
	}
	for key := range mp {
		delete(mp, key)
	}
	for i := uint32(0); i < sz; i++ {
		var key string
		var val interface{}
		key, err = m.ReadString()
		if err != nil {
			return err
		}
		val, err = m.ReadIntf()
		if err != nil {
			return err
		}
		mp[key] = val
	}
	return nil
}

// ReadTime reads a time.Time object from the reader.
// The returned time's location will be set to time.Local.
func (m *Reader) ReadTime() (time.Time, error) {
	p, err := m.R.Peek(15)
	if err != nil {
		return time.Time{}, err
	}
	if p[0] != mext8 || p[1] != 12 {
		return time.Time{}, badPrefix(TimeType, p[0])
	}
	if int8(p[2]) != TimeExtension {
		return time.Time{}, errExt(int8(p[2]), TimeExtension)
	}
	sec, nsec := getUnix(p[3:])
	t := time.Unix(sec, int64(nsec)).Local()
	_, err = m.R.Skip(15)
	return t, err
}

// ReadIntf reads out the next object as a raw interface{}. Arrays are decoded as []interface{},
// and maps are decoded as map[string]interface{}. Integers are decoded as int64, and unsigned
// integers are decoded as uint64.
func (m *Reader) ReadIntf() (interface{}, error) {
	t, err := m.NextType()
	if err != nil {
		return nil, err
	}
	switch t {
	case BoolType:
		return m.ReadBool()
	case IntType:
		return m.ReadInt64()
	case UintType:
		return m.ReadUint64()
	case BinType:
		return m.ReadBytes(nil)
	case StrType:
		return m.ReadString()
	case Complex64Type:
		return m.ReadComplex64()
	case Complex128Type:
		return m.ReadComplex128()
	case TimeType:
		return m.ReadTime()
	case ExtensionType:
		tt, err := m.peekExtensionType()
		if err != nil {
			return nil, err
		}
		f, ok := extensionReg[tt]
		if ok {
			e := f()
			err = m.ReadExtension(e)
			return e, err
		}
		e := &RawExtension{Type: tt}
		err = m.ReadExtension(e)
		return e, err
	case MapType:
		mp := make(map[string]interface{})
		err = m.ReadMapStrIntf(mp)
		return mp, err
	case NilType:
		return nil, m.ReadNil()
	case Float32Type:
		return m.ReadFloat32()
	case Float64Type:
		return m.ReadFloat64()
	case ArrayType:
		sz, err := m.ReadArrayHeader()
		if err != nil {
			return nil, err
		}
		out := make([]interface{}, int(sz))
		for j := range out {
			out[j], err = m.ReadIntf()
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	default:
		return nil, fatal // unreachable
	}
}
