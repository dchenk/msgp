package msgp

import (
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"time"
)

// Sizer is an interface implemented by types that can estimate their size when
// encoded to MessagePack. This interface is optional, but encoding/marshaling
// implementations may use this as a way to pre-allocate memory for serialization.
type Sizer interface {
	Msgsize() int
}

var btsType = reflect.TypeOf(([]byte)(nil))

// Nowhere is an io.Writer to nowhere (used by generated tests).
var Nowhere = nwhere{}

type nwhere struct{}

func (nwhere) Write(p []byte) (int, error) { return len(p), nil }

// Marshaler is the interface implemented by types that know how to marshal themselves
// as MessagePack. MarshalMsg appends the marshalled form of the object to the provided
// byte slice, returning the extended slice and any errors encountered.
type Marshaler interface {
	MarshalMsg([]byte) ([]byte, error)
}

// Encoder is the interface implemented by types that know how to write themselves
// as MessagePack using a *msgp.Writer.
type Encoder interface {
	EncodeMsg(*Writer) error
}

// MarshalSizer combines the Marshaler and Sizer interfaces.
type MarshalSizer interface {
	Marshaler
	Sizer
}

// Writer is a buffered writer that can be used to write MessagePack objects to an io.Writer.
// You must call *Writer.Flush() to flush all of the buffered data to the underlying writer.
type Writer struct {
	w    io.Writer
	buf  []byte
	wLoc int // The index at which to write.
}

// NewWriter creates a new Writer.
func NewWriter(w io.Writer) *Writer {
	if wr, ok := w.(*Writer); ok && w != nil {
		return wr
	}
	return &Writer{
		buf: make([]byte, 2048),
		w:   w,
	}
}

// NewWriterSize creates a Writer with a custom buffer size.
func NewWriterSize(w io.Writer, sz int) *Writer {
	// We must be able to require() 18 contiguous bytes for WriteComplex128,
	// so that is the practical minimal buffer size.
	if sz < 18 {
		sz = 18
	}
	return &Writer{
		w:   w,
		buf: make([]byte, sz),
	}
}

// Encode encodes an Encoder to any io.Writer.
func Encode(w io.Writer, e Encoder) error {
	wr := NewWriter(w)
	err := e.EncodeMsg(wr)
	if err == nil {
		err = wr.Flush()
	}
	return err
}

// Flush flushes all of the buffered data to the underlying writer.
func (mw *Writer) Flush() error {
	if mw.wLoc == 0 {
		return nil
	}
	n, err := mw.w.Write(mw.buf[:mw.wLoc])
	if err != nil {
		if n > 0 {
			mw.wLoc = copy(mw.buf, mw.buf[n:mw.wLoc])
		}
		return err
	}
	if n < mw.wLoc {
		return io.ErrShortWrite
	}
	mw.wLoc = 0
	return nil
}

// OpenSpace returns the number of bytes currently free for writing to the write buffer.
func (mw *Writer) OpenSpace() int { return len(mw.buf) - mw.wLoc }

// require ensures that, starting at the index returned as an int here, you can
// immediately write n bytes to the buffer.
//
// Important: This function must only be called with a number that is guaranteed
// to be less than len(mw.buf). Typically, it is called with a constant.
func (mw *Writer) require(n int) (int, error) {
	wl := mw.wLoc
	if mw.OpenSpace() < n {
		if err := mw.Flush(); err != nil {
			return 0, err
		}
		wl = mw.wLoc
	}
	mw.wLoc += n
	return wl, nil
}

// Append can be used to append a few (no more than the total buffer length) single
// bytes to the buffer.
func (mw *Writer) Append(bts ...byte) error {
	if mw.OpenSpace() < len(bts) {
		if err := mw.Flush(); err != nil {
			return err
		}
	}
	mw.wLoc += copy(mw.buf[mw.wLoc:], bts)
	return nil
}

// push pushes one byte onto the buffer.
func (mw *Writer) push(b byte) error {
	if mw.wLoc == len(mw.buf) {
		if err := mw.Flush(); err != nil {
			return err
		}
	}
	mw.buf[mw.wLoc] = b
	mw.wLoc++
	return nil
}

func (mw *Writer) prefix8(b byte, u uint8) error {
	const need = 2
	if mw.OpenSpace() < need {
		if err := mw.Flush(); err != nil {
			return err
		}
	}
	prefixu8(mw.buf[mw.wLoc:], b, u)
	mw.wLoc += need
	return nil
}

func (mw *Writer) prefix16(b byte, u uint16) error {
	const need = 3
	if mw.OpenSpace() < need {
		if err := mw.Flush(); err != nil {
			return err
		}
	}
	prefixu16(mw.buf[mw.wLoc:], b, u)
	mw.wLoc += need
	return nil
}

func (mw *Writer) prefix32(b byte, u uint32) error {
	const need = 5
	if mw.OpenSpace() < need {
		if err := mw.Flush(); err != nil {
			return err
		}
	}
	prefixu32(mw.buf[mw.wLoc:], b, u)
	mw.wLoc += need
	return nil
}

func (mw *Writer) prefix64(b byte, u uint64) error {
	const need = 9
	if mw.OpenSpace() < need {
		if err := mw.Flush(); err != nil {
			return err
		}
	}
	prefixu64(mw.buf[mw.wLoc:], b, u)
	mw.wLoc += need
	return nil
}

// Write implements io.Writer to write directly to the buffer.
func (mw *Writer) Write(p []byte) (int, error) {
	l := len(p)
	if mw.OpenSpace() < l {
		if err := mw.Flush(); err != nil {
			return 0, err
		}
		if l > len(mw.buf) {
			return mw.w.Write(p)
		}
	}
	mw.wLoc += copy(mw.buf[mw.wLoc:], p)
	return l, nil
}

// writeString writes s to the buffer.
func (mw *Writer) writeString(s string) error {
	l := len(s)
	if mw.OpenSpace() < l {
		if err := mw.Flush(); err != nil {
			return err
		}
		if l > len(mw.buf) {
			n, err := io.WriteString(mw.w, s)
			if err != nil {
				return err
			}
			if n < l {
				return io.ErrShortWrite
			}
			return nil
		}
	}
	mw.wLoc += copy(mw.buf[mw.wLoc:], s)
	return nil
}

// Reset resets the underlying buffer used by the Writer.
func (mw *Writer) Reset(w io.Writer) {
	mw.buf = mw.buf[:cap(mw.buf)]
	mw.w = w
	mw.wLoc = 0
}

// WriteMapHeader writes a map header of the given size to the buffer.
func (mw *Writer) WriteMapHeader(sz uint32) error {
	switch {
	case sz <= 15:
		return mw.push(wfixmap(uint8(sz)))
	case sz <= math.MaxUint16:
		return mw.prefix16(mmap16, uint16(sz))
	default:
		return mw.prefix32(mmap32, sz)
	}
}

// WriteArrayHeader writes an array header of the given size to the buffer.
func (mw *Writer) WriteArrayHeader(sz uint32) error {
	switch {
	case sz <= 15:
		return mw.push(wfixarray(uint8(sz)))
	case sz <= math.MaxUint16:
		return mw.prefix16(marray16, uint16(sz))
	default:
		return mw.prefix32(marray32, sz)
	}
}

// WriteNil writes a nil byte to the buffer.
func (mw *Writer) WriteNil() error {
	return mw.push(mnil)
}

// WriteFloat64 writes a float64 to the writer
func (mw *Writer) WriteFloat64(f float64) error {
	return mw.prefix64(mfloat64, math.Float64bits(f))
}

// WriteFloat32 writes a float32 to the writer
func (mw *Writer) WriteFloat32(f float32) error {
	return mw.prefix32(mfloat32, math.Float32bits(f))
}

// WriteInt64 writes an int64 to the writer.
func (mw *Writer) WriteInt64(i int64) error {
	if i >= 0 {
		switch {
		case i <= math.MaxInt8:
			return mw.push(wfixint(uint8(i)))
		case i <= math.MaxInt16:
			return mw.prefix16(mint16, uint16(i))
		case i <= math.MaxInt32:
			return mw.prefix32(mint32, uint32(i))
		default:
			return mw.prefix64(mint64, uint64(i))
		}
	}
	switch {
	case i >= -32:
		return mw.push(wnfixint(int8(i)))
	case i >= math.MinInt8:
		return mw.prefix8(mint8, uint8(i))
	case i >= math.MinInt16:
		return mw.prefix16(mint16, uint16(i))
	case i >= math.MinInt32:
		return mw.prefix32(mint32, uint32(i))
	default:
		return mw.prefix64(mint64, uint64(i))
	}
}

// WriteInt8 writes an int8 to the writer.
func (mw *Writer) WriteInt8(i int8) error {
	if i >= 0 {
		return mw.push(wfixint(uint8(i)))
	}
	if i >= -32 {
		return mw.push(wnfixint(i))
	}
	return mw.prefix8(mint8, uint8(i))
}

// WriteInt16 writes an int16 to the writer.
func (mw *Writer) WriteInt16(i int16) error { return mw.WriteInt64(int64(i)) }

// WriteInt32 writes an int32 to the writer.
func (mw *Writer) WriteInt32(i int32) error { return mw.WriteInt64(int64(i)) }

// WriteInt writes an int to the writer.
func (mw *Writer) WriteInt(i int) error { return mw.WriteInt64(int64(i)) }

// WriteUint64 writes a uint64 to the writer.
func (mw *Writer) WriteUint64(u uint64) error {
	switch {
	case u <= math.MaxInt8:
		return mw.push(wfixint(uint8(u)))
	case u <= math.MaxUint8:
		return mw.prefix8(muint8, uint8(u))
	case u <= math.MaxUint16:
		return mw.prefix16(muint16, uint16(u))
	case u <= math.MaxUint32:
		return mw.prefix32(muint32, uint32(u))
	default:
		return mw.prefix64(muint64, u)
	}
}

// WriteUint8 writes a uint8 to the writer.
func (mw *Writer) WriteUint8(u uint8) error {
	if u <= math.MaxInt8 {
		return mw.push(wfixint(u))
	}
	return mw.prefix8(muint8, u)
}

// WriteUint16 writes a uint16 to the writer.
func (mw *Writer) WriteUint16(u uint16) error { return mw.WriteUint64(uint64(u)) }

// WriteUint32 writes a uint32 to the writer.
func (mw *Writer) WriteUint32(u uint32) error { return mw.WriteUint64(uint64(u)) }

// WriteUint writes a uint to the writer.
func (mw *Writer) WriteUint(u uint) error { return mw.WriteUint64(uint64(u)) }

// WriteByte does the same thing as WriteUint8.
func (mw *Writer) WriteByte(u byte) error { return mw.WriteUint8(u) }

// WriteBool writes a bool to the writer.
func (mw *Writer) WriteBool(b bool) error {
	if b {
		return mw.push(mtrue)
	}
	return mw.push(mfalse)
}

// WriteBytes writes binary data as 'bin' to the writer.
func (mw *Writer) WriteBytes(b []byte) error {
	err := mw.WriteBytesHeader(uint32(len(b)))
	if err != nil {
		return err
	}
	_, err = mw.Write(b)
	return err
}

// WriteBytesHeader writes just the size header df a MessagePack 'bin' object.
// The user is responsible for then writing sz more bytes into the stream.
func (mw *Writer) WriteBytesHeader(sz uint32) error {
	switch {
	case sz <= math.MaxUint8:
		return mw.prefix8(mbin8, uint8(sz))
	case sz <= math.MaxUint16:
		return mw.prefix16(mbin16, uint16(sz))
	default:
		return mw.prefix32(mbin32, sz)
	}
}

// WriteString writes a MessagePack string to the writer.
// (This is NOT an implementation of io.StringWriter)
func (mw *Writer) WriteString(s string) error {
	err := mw.WriteStringHeader(uint32(len(s)))
	if err != nil {
		return err
	}
	return mw.writeString(s)
}

// WriteStringHeader writes just the string size header of a MessagePack 'str' object.
// The user is responsible for writing sz more valid UTF-8 bytes to the stream.
func (mw *Writer) WriteStringHeader(sz uint32) error {
	switch {
	case sz <= 31:
		return mw.push(wfixstr(uint8(sz)))
	case sz <= math.MaxUint8:
		return mw.prefix8(mstr8, uint8(sz))
	case sz <= math.MaxUint16:
		return mw.prefix16(mstr16, uint16(sz))
	default:
		return mw.prefix32(mstr32, sz)
	}
}

// WriteStringFromBytes writes a 'str' object from a []byte representing a string.
func (mw *Writer) WriteStringFromBytes(str []byte) error {
	err := mw.WriteStringHeader(uint32(len(str)))
	if err != nil {
		return err
	}
	_, err = mw.Write(str)
	return err
}

// WriteComplex64 writes a complex64 to the writer.
func (mw *Writer) WriteComplex64(f complex64) error {
	i, err := mw.require(10)
	if err != nil {
		return err
	}
	mw.buf[i] = mfixext8
	mw.buf[i+1] = Complex64Extension
	big.PutUint32(mw.buf[i+2:], math.Float32bits(real(f)))
	big.PutUint32(mw.buf[i+6:], math.Float32bits(imag(f)))
	return nil
}

// WriteComplex128 writes a complex128 to the writer
func (mw *Writer) WriteComplex128(f complex128) error {
	i, err := mw.require(18)
	if err != nil {
		return err
	}
	mw.buf[i] = mfixext16
	mw.buf[i+1] = Complex128Extension
	big.PutUint64(mw.buf[i+2:], math.Float64bits(real(f)))
	big.PutUint64(mw.buf[i+10:], math.Float64bits(imag(f)))
	return nil
}

// WriteMapStrStr writes a map[string]string to the writer
func (mw *Writer) WriteMapStrStr(mp map[string]string) (err error) {
	err = mw.WriteMapHeader(uint32(len(mp)))
	if err != nil {
		return
	}
	for key, val := range mp {
		err = mw.WriteString(key)
		if err != nil {
			return
		}
		err = mw.WriteString(val)
		if err != nil {
			return
		}
	}
	return
}

// WriteMapStrIntf writes a map[string]interface to the writer
func (mw *Writer) WriteMapStrIntf(mp map[string]interface{}) (err error) {
	err = mw.WriteMapHeader(uint32(len(mp)))
	if err != nil {
		return
	}
	for key, val := range mp {
		err = mw.WriteString(key)
		if err != nil {
			return
		}
		err = mw.WriteIntf(val)
		if err != nil {
			return
		}
	}
	return
}

// WriteTime writes a time.Time object to the wire.
//
// Time is encoded as Unix time, which means that location (time zone) data is removed from the object.
// The encoded object itself is 12 bytes: 8 bytes for a big-endian 64-bit integer denoting seconds
// elapsed since "zero" Unix time, followed by 4 bytes for a big-endian 32-bit signed integer denoting
// the nanosecond offset of the time. This encoding is intended to ease portability across languages.
func (mw *Writer) WriteTime(t time.Time) error {
	t = t.UTC()
	i, err := mw.require(15)
	if err != nil {
		return err
	}
	mw.buf[i] = mext8
	mw.buf[i+1] = 12
	mw.buf[i+2] = TimeExtension
	putUnix(mw.buf[i+3:], t.Unix(), int32(t.Nanosecond()))
	return nil
}

// WriteIntf writes the concrete type of v. The type of v must
// be one of the following:
//  - bool, float, string, []byte, int, uint, complex, time.Time, or nil
//  - map of supported types, with string keys
//  - array or slice of supported types
//  - pointer to a supported type
//  - type that implements the msgp.Encoder interface
//  - type that implements the msgp.Extension interface
func (mw *Writer) WriteIntf(v interface{}) error {

	if v == nil {
		return mw.WriteNil()
	}

	switch v := v.(type) {

	// preferred interfaces
	case Encoder:
		return v.EncodeMsg(mw)
	case Extension:
		return mw.WriteExtension(v)

	// concrete types
	case bool:
		return mw.WriteBool(v)
	case float32:
		return mw.WriteFloat32(v)
	case float64:
		return mw.WriteFloat64(v)
	case complex64:
		return mw.WriteComplex64(v)
	case complex128:
		return mw.WriteComplex128(v)
	case uint8:
		return mw.WriteUint8(v)
	case uint16:
		return mw.WriteUint16(v)
	case uint32:
		return mw.WriteUint32(v)
	case uint64:
		return mw.WriteUint64(v)
	case uint:
		return mw.WriteUint(v)
	case int8:
		return mw.WriteInt8(v)
	case int16:
		return mw.WriteInt16(v)
	case int32:
		return mw.WriteInt32(v)
	case int64:
		return mw.WriteInt64(v)
	case int:
		return mw.WriteInt(v)
	case string:
		return mw.WriteString(v)
	case []byte:
		return mw.WriteBytes(v)
	case map[string]string:
		return mw.WriteMapStrStr(v)
	case map[string]interface{}:
		return mw.WriteMapStrIntf(v)
	case time.Time:
		return mw.WriteTime(v)
	}

	val := reflect.ValueOf(v)
	if !isSupported(val.Kind()) || !val.IsValid() {
		return fmt.Errorf("msgp: type %s not supported", val)
	}

	switch val.Kind() {
	case reflect.Ptr:
		if val.IsNil() {
			return mw.WriteNil()
		}
		return mw.WriteIntf(val.Elem().Interface())
	case reflect.Slice:
		return mw.writeSlice(val)
	case reflect.Map:
		return mw.writeMap(val)
	}
	return &ErrUnsupportedType{val.Type()}
}

func (mw *Writer) writeMap(v reflect.Value) error {
	if v.Type().Key().Kind() != reflect.String {
		return errors.New("msgp: map keys must be strings")
	}
	ks := v.MapKeys()
	err := mw.WriteMapHeader(uint32(len(ks)))
	if err != nil {
		return err
	}
	for _, key := range ks {
		val := v.MapIndex(key)
		err = mw.WriteString(key.String())
		if err != nil {
			return err
		}
		err = mw.WriteIntf(val.Interface())
		if err != nil {
			return err
		}
	}
	return nil
}

func (mw *Writer) writeSlice(v reflect.Value) error {
	if v.Type().ConvertibleTo(btsType) { // is []byte
		return mw.WriteBytes(v.Bytes())
	}
	sz := uint32(v.Len())
	err := mw.WriteArrayHeader(sz)
	if err != nil {
		return err
	}
	for i := uint32(0); i < sz; i++ {
		err = mw.WriteIntf(v.Index(int(i)).Interface())
		if err != nil {
			return err
		}
	}
	return nil
}

func (mw *Writer) writeStruct(v reflect.Value) error {
	if enc, ok := v.Interface().(Encoder); ok {
		return enc.EncodeMsg(mw)
	}
	return fmt.Errorf("msgp: unsupported type: %s", v.Type())
}

// isSupported says if k is encodable.
func isSupported(k reflect.Kind) bool {
	return k != reflect.Func && k != reflect.Chan && k != reflect.Invalid && k != reflect.UnsafePointer
}

// GuessSize guesses the size of the underlying value of 'i'. If the underlying value is not
// a simple builtin (or []byte), GuessSize defaults to 512.
func GuessSize(i interface{}) int {
	if i == nil {
		return NilSize
	}
	switch i := i.(type) {
	case Sizer:
		return i.Msgsize()
	case Extension:
		return ExtensionPrefixSize + i.Len()
	case float64:
		return Float64Size
	case float32:
		return Float32Size
	case uint8, uint16, uint32, uint64, uint:
		return UintSize
	case int8, int16, int32, int64, int:
		return IntSize
	case []byte:
		return BytesPrefixSize + len(i)
	case string:
		return StringPrefixSize + len(i)
	case complex64:
		return Complex64Size
	case complex128:
		return Complex128Size
	case bool:
		return BoolSize
	case map[string]interface{}:
		s := MapHeaderSize
		for key, val := range i {
			s += StringPrefixSize + len(key) + GuessSize(val)
		}
		return s
	case map[string]string:
		s := MapHeaderSize
		for key, val := range i {
			s += 2*StringPrefixSize + len(key) + len(val)
		}
		return s
	default:
		return 512
	}
}
