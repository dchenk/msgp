package msgp

import (
	"io/ioutil"
	"os"
)

// ReadFile reads from file into dst.
func ReadFile(dst Unmarshaler, file *os.File) error {
	if u, ok := dst.(Decoder); ok {
		return u.DecodeMsg(NewReader(file))
	}
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	_, err = dst.UnmarshalMsg(data)
	return err
}

// WriteFile writes from src into file.
func WriteFile(src MarshalSizer, file *os.File) error {
	if e, ok := src.(Encoder); ok {
		w := NewWriter(file)
		err := e.EncodeMsg(w)
		if err == nil {
			err = w.Flush()
		}
		return err
	}
	raw, err := src.MarshalMsg(nil)
	if err != nil {
		return err
	}
	_, err = file.Write(raw)
	return err
}
