package msgp

import "testing"

func TestNewEndlessReader(t *testing.T) {
	er := NewEndlessReader(nil, &testing.B{})
	if er == nil {
		t.Error("reader is nil")
	}
}

func TestEndlessReader_Read(t *testing.T) {
	// TODO
}
