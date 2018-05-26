package msgp

import (
	"fmt"
	"math"
)

const (
	// Complex64Extension represents an extension for complex64 numbers.
	Complex64Extension = 3

	// Complex128Extension represents an extension for complex128 numbers.
	Complex128Extension = 4

	// TimeExtension represents an extension for timestamps. This is not the timestamp format
	// defined in the MessagePack specification.
	TimeExtension = 5
)

// extensionReg contains registered extensions.
var extensionReg = make(map[int8]func() Extension)

// RegisterExtension registers extensions so that they can be initialized and returned
// by methods that decode `interface{}` values. This should only be called during
// initialization. Func f should return a newly-initialized zero value of the extension.
// Keep in mind that extensions 3, 4, and 5 are reserved for complex64, complex128, and
// time.Time, respectively, and that MessagePack reserves extension types from -127 to -1.
//
// For example, if you wanted to register a user-defined struct:
//
//  msgp.RegisterExtension(10, func() msgp.Extension { &MyExtension{} })
//
// RegisterExtension will panic if you call it multiple times with the same 'typ' argument
// or if you use a reserved type (3, 4, or 5).
func RegisterExtension(typ int8, f func() Extension) {
	if typ == Complex64Extension || typ == Complex128Extension || typ == TimeExtension {
		panic(fmt.Sprint("msgp: forbidden extension type:", typ))
	}
	if _, ok := extensionReg[typ]; ok {
		panic(fmt.Sprint("msgp: RegisterExtension() called with typ", typ, "more than once"))
	}
	extensionReg[typ] = f
}

// ExtensionTypeError is an error type returned when there is a mis-match between an extension
// type and the type encoded on the wire.
type ExtensionTypeError struct {
	Got, Want int8
}

// Error implements the error interface.
func (e ExtensionTypeError) Error() string {
	return fmt.Sprintf("msgp: error decoding extension: wanted type %d; got type %d", e.Want, e.Got)
}

// Resumable returns true for ExtensionTypeError errors.
func (e ExtensionTypeError) Resumable() bool { return true }

func errExt(got int8, wanted int8) error {
	return ExtensionTypeError{got, wanted}
}

// Extension is the interface implemented by types that want to define their own binary encoding.
type Extension interface {
	// ExtensionType returns an int8 that identifies the extension's concrete type.
	// (Types below zero are reserved by the MessagePack specification.)
	ExtensionType() int8

	// Len returns the length of the data to be encoded.
	Len() int

	// MarshalBinaryTo copies the data into the supplied slice, assuming that the
	// slice has length Len().
	MarshalBinaryTo([]byte) error

	// UnmarshalBinary unmarshals the slice into the object.
	UnmarshalBinary([]byte) error
}

// RawExtension implements the Extension interface.
type RawExtension struct {
	Data []byte
	Type int8
}

// ExtensionType implements Extension.ExtensionType, and returns r.Type
func (r *RawExtension) ExtensionType() int8 { return r.Type }

// Len implements Extension.Len.
func (r *RawExtension) Len() int { return len(r.Data) }

// MarshalBinaryTo implements Extension.MarshalBinaryTo and returns a copy of r.Data.
func (r *RawExtension) MarshalBinaryTo(d []byte) error {
	copy(d, r.Data)
	return nil
}

// UnmarshalBinary implements Extension.UnmarshalBinary and sets r.Data to the contents
// of the provided slice.
func (r *RawExtension) UnmarshalBinary(b []byte) error {
	if cap(r.Data) >= len(b) {
		r.Data = r.Data[0:len(b)]
	} else {
		r.Data = make([]byte, len(b))
	}
	copy(r.Data, b)
	return nil
}

// WriteExtension writes an extension type to the writer.
func (mw *Writer) WriteExtension(e Extension) error {
	l := e.Len()
	switch l {
	case 0:
		i, err := mw.require(3)
		if err != nil {
			return err
		}
		mw.buf[i] = mext8
		mw.buf[i+1] = 0
		mw.buf[i+2] = byte(e.ExtensionType())
	case 1:
		i, err := mw.require(2)
		if err != nil {
			return err
		}
		mw.buf[i] = mfixext1
		mw.buf[i+1] = byte(e.ExtensionType())
	case 2:
		i, err := mw.require(2)
		if err != nil {
			return err
		}
		mw.buf[i] = mfixext2
		mw.buf[i+1] = byte(e.ExtensionType())
	case 4:
		i, err := mw.require(2)
		if err != nil {
			return err
		}
		mw.buf[i] = mfixext4
		mw.buf[i+1] = byte(e.ExtensionType())
	case 8:
		i, err := mw.require(2)
		if err != nil {
			return err
		}
		mw.buf[i] = mfixext8
		mw.buf[i+1] = byte(e.ExtensionType())
	case 16:
		i, err := mw.require(2)
		if err != nil {
			return err
		}
		mw.buf[i] = mfixext16
		mw.buf[i+1] = byte(e.ExtensionType())
	default:
		switch {
		case l < math.MaxUint8:
			i, err := mw.require(3)
			if err != nil {
				return err
			}
			mw.buf[i] = mext8
			mw.buf[i+1] = byte(uint8(l))
			mw.buf[i+2] = byte(e.ExtensionType())
		case l < math.MaxUint16:
			i, err := mw.require(4)
			if err != nil {
				return err
			}
			mw.buf[i] = mext16
			big.PutUint16(mw.buf[i+1:], uint16(l))
			mw.buf[i+3] = byte(e.ExtensionType())
		default:
			i, err := mw.require(6)
			if err != nil {
				return err
			}
			mw.buf[i] = mext32
			big.PutUint32(mw.buf[i+1:], uint32(l))
			mw.buf[i+5] = byte(e.ExtensionType())
		}
	}
	// We can only write directly to the buffer if we're sure that it
	// fits the object.
	if l <= len(mw.buf) {
		i, err := mw.require(l)
		if err != nil {
			return err
		}
		return e.MarshalBinaryTo(mw.buf[i:])
	}
	// Here we create a new buffer just large enough for the body
	// and save it as the write buffer.
	err := mw.Flush()
	if err != nil {
		return err
	}
	buf := make([]byte, l)
	err = e.MarshalBinaryTo(buf)
	if err != nil {
		return err
	}
	mw.buf = buf
	mw.wLoc = l
	return nil
}

// peek at the extension type, assuming the next kind to be read is Extension.
func (m *Reader) peekExtensionType() (int8, error) {
	p, err := m.R.Peek(2)
	if err != nil {
		return 0, err
	}
	spec := sizes[p[0]]
	if spec.typ != ExtensionType {
		return 0, badPrefix(ExtensionType, p[0])
	}
	if spec.extra == constsize {
		return int8(p[1]), nil
	}
	size := spec.size
	p, err = m.R.Peek(int(size))
	if err != nil {
		return 0, err
	}
	return int8(p[size-1]), nil
}

// peekExtension peeks at the extension encoding type
// (must guarantee at least 1 byte in 'b')
func peekExtension(b []byte) (int8, error) {
	spec := sizes[b[0]]
	size := spec.size
	if spec.typ != ExtensionType {
		return 0, badPrefix(ExtensionType, b[0])
	}
	if len(b) < int(size) {
		return 0, ErrShortBytes
	}
	// For fixed extensions, the type information is in
	// the second byte.
	if spec.extra == constsize {
		return int8(b[1]), nil
	}
	// Otherwise, it's in the last part of the prefix.
	return int8(b[size-1]), nil
}

// ReadExtension reads the next object from the reader as an extension. ReadExtension will fail if
// the next object in the stream is not an extension or if e.Type() is not the same as the wire type.
func (m *Reader) ReadExtension(e Extension) error {

	p, err := m.R.Peek(2)
	if err != nil {
		return err
	}
	lead := p[0]
	var read, off int
	switch lead {
	case mfixext1:
		if int8(p[1]) != e.ExtensionType() {
			return errExt(int8(p[1]), e.ExtensionType())
		}
		p, err = m.R.Peek(3)
		if err != nil {
			return err
		}
		err = e.UnmarshalBinary(p[2:])
		if err == nil {
			_, err = m.R.Skip(3)
		}
		return err

	case mfixext2:
		if int8(p[1]) != e.ExtensionType() {
			return errExt(int8(p[1]), e.ExtensionType())
		}
		p, err = m.R.Peek(4)
		if err != nil {
			return err
		}
		err = e.UnmarshalBinary(p[2:])
		if err == nil {
			_, err = m.R.Skip(4)
		}
		return err

	case mfixext4:
		if int8(p[1]) != e.ExtensionType() {
			return errExt(int8(p[1]), e.ExtensionType())
		}
		p, err = m.R.Peek(6)
		if err != nil {
			return err
		}
		err = e.UnmarshalBinary(p[2:])
		if err == nil {
			_, err = m.R.Skip(6)
		}
		return err

	case mfixext8:
		if int8(p[1]) != e.ExtensionType() {
			return errExt(int8(p[1]), e.ExtensionType())
		}
		p, err = m.R.Peek(10)
		if err != nil {
			return err
		}
		err = e.UnmarshalBinary(p[2:])
		if err == nil {
			_, err = m.R.Skip(10)
		}
		return err

	case mfixext16:
		if int8(p[1]) != e.ExtensionType() {
			return errExt(int8(p[1]), e.ExtensionType())
		}
		p, err = m.R.Peek(18)
		if err != nil {
			return err
		}
		err = e.UnmarshalBinary(p[2:])
		if err == nil {
			_, err = m.R.Skip(18)
		}
		return err

	case mext8:
		p, err = m.R.Peek(3)
		if err != nil {
			return err
		}
		if int8(p[2]) != e.ExtensionType() {
			return errExt(int8(p[2]), e.ExtensionType())
		}
		read = int(uint8(p[1]))
		off = 3

	case mext16:
		p, err = m.R.Peek(4)
		if err != nil {
			return err
		}
		if int8(p[3]) != e.ExtensionType() {
			return errExt(int8(p[3]), e.ExtensionType())
		}
		read = int(big.Uint16(p[1:]))
		off = 4

	case mext32:
		p, err = m.R.Peek(6)
		if err != nil {
			return err
		}
		if int8(p[5]) != e.ExtensionType() {
			return errExt(int8(p[5]), e.ExtensionType())
		}
		read = int(big.Uint32(p[1:]))
		off = 6

	default:
		return badPrefix(ExtensionType, lead)
	}

	p, err = m.R.Peek(read + off)
	if err != nil {
		return err
	}
	err = e.UnmarshalBinary(p[off:])
	if err == nil {
		_, err = m.R.Skip(read + off)
	}
	return err

}

// AppendExtension appends a MessagePack extension to slice b.
func AppendExtension(b []byte, e Extension) ([]byte, error) {
	l := e.Len()
	var n int
	switch l {
	case 0:
		b, n = ensure(b, 3)
		b[n] = mext8
		b[n+1] = 0
		b[n+2] = byte(e.ExtensionType())
		return b[:n+3], nil
	case 1:
		b, n = ensure(b, 3)
		b[n] = mfixext1
		b[n+1] = byte(e.ExtensionType())
		n += 2
	case 2:
		b, n = ensure(b, 4)
		b[n] = mfixext2
		b[n+1] = byte(e.ExtensionType())
		n += 2
	case 4:
		b, n = ensure(b, 6)
		b[n] = mfixext4
		b[n+1] = byte(e.ExtensionType())
		n += 2
	case 8:
		b, n = ensure(b, 10)
		b[n] = mfixext8
		b[n+1] = byte(e.ExtensionType())
		n += 2
	case 16:
		b, n = ensure(b, 18)
		b[n] = mfixext16
		b[n+1] = byte(e.ExtensionType())
		n += 2
	}
	switch {
	case l < math.MaxUint8:
		b, n = ensure(b, l+3)
		b[n] = mext8
		b[n+1] = byte(uint8(l))
		b[n+2] = byte(e.ExtensionType())
		n += 3
	case l < math.MaxUint16:
		b, n = ensure(b, l+4)
		b[n] = mext16
		big.PutUint16(b[n+1:], uint16(l))
		b[n+3] = byte(e.ExtensionType())
		n += 4
	default:
		b, n = ensure(b, l+6)
		b[n] = mext32
		big.PutUint32(b[n+1:], uint32(l))
		b[n+5] = byte(e.ExtensionType())
		n += 6
	}
	return b, e.MarshalBinaryTo(b[n:])
}

// ReadExtensionBytes reads an extension from b into e and returns any remaining bytes.
// Possible errors:
// - ErrShortBytes ('b' not long enough)
// - ExtensionTypeError{} (wire type not the same as e.Type())
// - TypeError{} (next object not an extension)
// - InvalidPrefixError
// - An unmarshal error returned from e.UnmarshalBinary
func ReadExtensionBytes(b []byte, e Extension) ([]byte, error) {
	l := len(b)
	if l < 3 {
		return b, ErrShortBytes
	}
	lead := b[0]
	var (
		sz  int // size of 'data'
		off int // offset of 'data'
		typ int8
	)
	switch lead {
	case mfixext1:
		typ = int8(b[1])
		sz = 1
		off = 2
	case mfixext2:
		typ = int8(b[1])
		sz = 2
		off = 2
	case mfixext4:
		typ = int8(b[1])
		sz = 4
		off = 2
	case mfixext8:
		typ = int8(b[1])
		sz = 8
		off = 2
	case mfixext16:
		typ = int8(b[1])
		sz = 16
		off = 2
	case mext8:
		sz = int(uint8(b[1]))
		typ = int8(b[2])
		off = 3
		if sz == 0 {
			return b[3:], e.UnmarshalBinary(b[3:3])
		}
	case mext16:
		if l < 4 {
			return b, ErrShortBytes
		}
		sz = int(big.Uint16(b[1:]))
		typ = int8(b[3])
		off = 4
	case mext32:
		if l < 6 {
			return b, ErrShortBytes
		}
		sz = int(big.Uint32(b[1:]))
		typ = int8(b[5])
		off = 6
	default:
		return b, badPrefix(ExtensionType, lead)
	}

	if typ != e.ExtensionType() {
		return b, errExt(typ, e.ExtensionType())
	}

	// The data of the extension starts at off and is sz bytes long.
	if len(b[off:]) < sz {
		return b, ErrShortBytes
	}
	tot := off + sz
	return b[tot:], e.UnmarshalBinary(b[off:tot])
}
