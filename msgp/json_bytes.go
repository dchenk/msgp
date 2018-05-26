package msgp

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"
	"time"
)

// UnmarshalAsJSON takes raw MessagePack data and writes it as JSON to w.
// If an error is returned, the bytes not unmarshalled will also be returned.
// If no errors are encountered, the length of the returned slice will be zero.
func UnmarshalAsJSON(w io.Writer, msg []byte) ([]byte, error) {
	var cast bool
	var dst jsWriter
	if jsw, ok := w.(jsWriter); ok {
		dst = jsw
		cast = true
	} else {
		dst = bufio.NewWriterSize(w, 512)
	}
	var err error
	for len(msg) > 0 {
		msg, _, err = writeNext(dst, msg, nil)
	}
	if !cast && err == nil {
		err = dst.(*bufio.Writer).Flush()
	}
	return msg, err
}

func writeNext(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	if len(msg) == 0 {
		return msg, scratch, ErrShortBytes
	}
	t := getType(msg[0])
	if t == ExtensionType {
		// The TimeExtension type is encoded the time.MarshalJSON way.
		et, err := peekExtension(msg)
		if err != nil {
			return nil, scratch, err
		}
		if et == TimeExtension {
			t = TimeType
		}
	}
	switch t {
	case InvalidType:
		return msg, scratch, InvalidPrefixError(msg[0])
	case StrType:
		return rwStringBytes(w, msg, scratch)
	case BinType:
		return rwBytesBytes(w, msg, scratch)
	case MapType:
		return rwMapBytes(w, msg, scratch)
	case ArrayType:
		return rwArrayBytes(w, msg, scratch)
	case Float64Type:
		return rwFloat64Bytes(w, msg, scratch)
	case Float32Type:
		return rwFloat32Bytes(w, msg, scratch)
	case BoolType:
		return rwBoolBytes(w, msg, scratch)
	case IntType:
		return rwIntBytes(w, msg, scratch)
	case UintType:
		return rwUintBytes(w, msg, scratch)
	case NilType:
		return rwNullBytes(w, msg, scratch)
	case ExtensionType, Complex64Type, Complex128Type:
		return rwExtensionBytes(w, msg, scratch)
	case TimeType:
		return rwTimeBytes(w, msg, scratch)
	default:
		return nil, msg, InvalidPrefixError(msg[0])
	}
}

func rwArrayBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	sz, msg, err := ReadArrayHeaderBytes(msg)
	if err != nil {
		return msg, scratch, err
	}
	err = w.WriteByte('[')
	if err != nil {
		return msg, scratch, err
	}
	for i := uint32(0); i < sz; i++ {
		if i != 0 {
			err = w.WriteByte(',')
			if err != nil {
				return msg, scratch, err
			}
		}
		msg, scratch, err = writeNext(w, msg, scratch)
		if err != nil {
			return msg, scratch, err
		}
	}
	err = w.WriteByte(']')
	return msg, scratch, err
}

func rwMapBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	sz, msg, err := ReadMapHeaderBytes(msg)
	if err != nil {
		return msg, scratch, err
	}
	err = w.WriteByte('{')
	if err != nil {
		return msg, scratch, err
	}
	for i := uint32(0); i < sz; i++ {
		if i != 0 {
			err = w.WriteByte(',')
			if err != nil {
				return msg, scratch, err
			}
		}
		msg, scratch, err = rwMapKeyBytes(w, msg, scratch)
		if err != nil {
			return msg, scratch, err
		}
		err = w.WriteByte(':')
		if err != nil {
			return msg, scratch, err
		}
		msg, scratch, err = writeNext(w, msg, scratch)
		if err != nil {
			return msg, scratch, err
		}
	}
	err = w.WriteByte('}')
	return msg, scratch, err
}

func rwMapKeyBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	msg, scratch, err := rwStringBytes(w, msg, scratch)
	if err != nil {
		if tperr, ok := err.(TypeError); ok && tperr.Encoded == BinType {
			return rwBytesBytes(w, msg, scratch)
		}
	}
	return msg, scratch, err
}

func rwStringBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	str, msg, err := ReadStringZC(msg)
	if err != nil {
		return msg, scratch, err
	}
	_, err = rwQuoted(w, str)
	return msg, scratch, err
}

func rwBytesBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	bts, msg, err := ReadBytesZC(msg)
	if err != nil {
		return msg, scratch, err
	}
	l := base64.StdEncoding.EncodedLen(len(bts))
	if cap(scratch) >= l {
		scratch = scratch[0:l]
	} else {
		scratch = make([]byte, l)
	}
	base64.StdEncoding.Encode(scratch, bts)
	err = w.WriteByte('"')
	if err != nil {
		return msg, scratch, err
	}
	_, err = w.Write(scratch)
	if err != nil {
		return msg, scratch, err
	}
	err = w.WriteByte('"')
	return msg, scratch, err
}

func rwNullBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	msg, err := ReadNilBytes(msg)
	if err != nil {
		return msg, scratch, err
	}
	_, err = w.Write(null)
	return msg, scratch, err
}

func rwBoolBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	b, msg, err := ReadBoolBytes(msg)
	if err != nil {
		return msg, scratch, err
	}
	if b {
		_, err = w.WriteString("true")
		return msg, scratch, err
	}
	_, err = w.WriteString("false")
	return msg, scratch, err
}

func rwIntBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	i, msg, err := ReadInt64Bytes(msg)
	if err != nil {
		return msg, scratch, err
	}
	scratch = strconv.AppendInt(scratch[0:0], i, 10)
	_, err = w.Write(scratch)
	return msg, scratch, err
}

func rwUintBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	u, msg, err := ReadUint64Bytes(msg)
	if err != nil {
		return msg, scratch, err
	}
	scratch = strconv.AppendUint(scratch[0:0], u, 10)
	_, err = w.Write(scratch)
	return msg, scratch, err
}

func rwFloat32Bytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	f, msg, err := ReadFloat32Bytes(msg)
	if err != nil {
		return msg, scratch, err
	}
	scratch = strconv.AppendFloat(scratch[:0], float64(f), 'f', -1, 32)
	_, err = w.Write(scratch)
	return msg, scratch, err
}

func rwFloat64Bytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	f, msg, err := ReadFloat64Bytes(msg)
	if err != nil {
		return msg, scratch, err
	}
	scratch = strconv.AppendFloat(scratch[:0], f, 'f', -1, 64)
	_, err = w.Write(scratch)
	return msg, scratch, err
}

func rwTimeBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {
	t, msg, err := ReadTimeBytes(msg)
	if err != nil {
		return msg, scratch, err
	}
	bts, err := t.MarshalJSON()
	if err != nil {
		return msg, scratch, err
	}
	_, err = w.Write(bts)
	return msg, scratch, err
}

// rwExtensionBytes writes out an extension. Values of type time.Time should be handled by rwTimeBytes.
func rwExtensionBytes(w jsWriter, msg []byte, scratch []byte) ([]byte, []byte, error) {

	et, err := peekExtension(msg)
	if err != nil {
		return msg, scratch, err
	}

	// If the extension is registered, use its canonical JSON form.
	if f, ok := extensionReg[et]; ok {
		e := f()
		msg, err = ReadExtensionBytes(msg, e)
		if err != nil {
			return msg, scratch, err
		}
		bts, err := json.Marshal(e)
		if err != nil {
			return msg, scratch, err
		}
		_, err = w.Write(bts)
		return msg, scratch, err
	}

	// otherwise, write `{"type": <num>, "data": "<base64data>"}`
	r := RawExtension{}
	r.Type = et
	msg, err = ReadExtensionBytes(msg, &r)
	if err != nil {
		return msg, scratch, err
	}
	scratch, err = writeExt(w, r, scratch)
	return msg, scratch, err

}

func writeExt(w jsWriter, r RawExtension, scratch []byte) ([]byte, error) {
	_, err := w.WriteString(`{"type":`)
	if err != nil {
		return scratch, err
	}
	scratch = strconv.AppendInt(scratch[0:0], int64(r.Type), 10)
	_, err = w.Write(scratch)
	if err != nil {
		return scratch, err
	}
	_, err = w.WriteString(`,"data":"`)
	if err != nil {
		return scratch, err
	}
	l := base64.StdEncoding.EncodedLen(len(r.Data))
	if cap(scratch) >= l {
		scratch = scratch[0:l]
	} else {
		scratch = make([]byte, l)
	}
	base64.StdEncoding.Encode(scratch, r.Data)
	_, err = w.Write(scratch)
	if err != nil {
		return scratch, err
	}
	_, err = w.WriteString(`"}`)
	return scratch, err
}
