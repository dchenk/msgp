package gen

import (
	"fmt"
	"io"

	"github.com/dchenk/msgp/msgp"
)

func encode(w io.Writer) *encodeGen {
	return &encodeGen{
		p: printer{w: w},
	}
}

type encodeGen struct {
	passes
	p    printer
	fuse []byte
}

func (e *encodeGen) Method() Method { return Encode }

func (e *encodeGen) Apply(dirs []string) error {
	return nil
}

func (e *encodeGen) writeAndCheck(typ string, argfmt string, arg interface{}) {
	e.p.printf("\nerr = en.Write%s(%s)", typ, fmt.Sprintf(argfmt, arg))
	e.p.print(errCheck)
}

func (e *encodeGen) fuseHook() {
	if len(e.fuse) > 0 {
		e.appendRaw(e.fuse)
		e.fuse = e.fuse[:0]
	}
}

func (e *encodeGen) Fuse(b []byte) {
	if len(e.fuse) > 0 {
		e.fuse = append(e.fuse, b...)
	} else {
		e.fuse = b
	}
}

func (e *encodeGen) Execute(p Elem) error {

	if !e.p.ok() {
		return e.p.err
	}
	p = e.applyAll(p)
	if p == nil {
		return nil
	}
	if !isPrintable(p) {
		return nil
	}

	e.p.comment("EncodeMsg implements msgp.Encoder")

	e.p.printf("\nfunc (%s %s) EncodeMsg(en *msgp.Writer) (err error) {", p.Varname(), imutMethodReceiver(p))
	next(e, p)
	e.p.nakedReturn()
	return e.p.err

}

func (e *encodeGen) gStruct(s *Struct) {
	if !e.p.ok() {
		return
	}
	if s.AsTuple {
		e.structAsTuple(s)
	} else {
		e.structAsMap(s)
	}
}

func (e *encodeGen) structAsTuple(s *Struct) {
	nfields := len(s.Fields)
	data := msgp.AppendArrayHeader(nil, uint32(nfields))
	e.p.printf("\n// array header, size %d", nfields)
	e.Fuse(data)
	if len(s.Fields) == 0 {
		e.fuseHook()
	}
	for i := range s.Fields {
		if !e.p.ok() {
			return
		}
		next(e, s.Fields[i].fieldElem)
	}
}

func (e *encodeGen) appendRaw(bts []byte) {
	e.p.print("\nerr = en.Append(")
	for i, b := range bts {
		if i > 0 {
			e.p.print(", ")
		}
		e.p.printf("0x%x", b)
	}
	e.p.print(")\nif err != nil { return }")
}

func (e *encodeGen) structAsMap(s *Struct) {
	nfields := len(s.Fields)
	data := msgp.AppendMapHeader(nil, uint32(nfields))
	e.p.printf("\n// map header, size %d", nfields)
	e.Fuse(data)
	if len(s.Fields) == 0 {
		e.fuseHook()
	}
	for i := range s.Fields {
		if !e.p.ok() {
			return
		}
		data = msgp.AppendString(nil, s.Fields[i].fieldTag)
		e.p.printf("\n// write %q", s.Fields[i].fieldTag)
		e.Fuse(data)
		next(e, s.Fields[i].fieldElem)
	}
}

func (e *encodeGen) gMap(m *Map) {
	if !e.p.ok() {
		return
	}
	e.fuseHook()
	vname := m.Varname()
	e.writeAndCheck(mapHeader, lenAsUint32, vname)

	e.p.printf("\nfor %s, %s := range %s {", m.KeyIndx, m.ValIndx, vname)
	e.writeAndCheck(stringTyp, literalFmt, m.KeyIndx)
	next(e, m.Value)
	e.p.closeBlock()
}

func (e *encodeGen) gPtr(s *Ptr) {
	if !e.p.ok() {
		return
	}
	e.fuseHook()
	e.p.printf("\nif %s == nil { err = en.WriteNil(); if err != nil { return; } } else {", s.Varname())
	next(e, s.Value)
	e.p.closeBlock()
}

func (e *encodeGen) gSlice(s *Slice) {
	if !e.p.ok() {
		return
	}
	e.fuseHook()
	e.writeAndCheck(arrayHeader, lenAsUint32, s.Varname())
	e.p.rangeBlock(s.Index, s.Varname(), e, s.Els)
}

func (e *encodeGen) gArray(a *Array) {
	if !e.p.ok() {
		return
	}
	e.fuseHook()
	// shortcut for [const]byte
	if be, ok := a.Els.(*BaseElem); ok && (be.Value == Byte || be.Value == Uint8) {
		e.p.printf("\nerr = en.WriteBytes((%s)[:])", a.Varname())
		e.p.print(errCheck)
		return
	}

	e.writeAndCheck(arrayHeader, literalFmt, coerceArraySize(a.Size))
	e.p.rangeBlock(a.Index, a.Varname(), e, a.Els)
}

func (e *encodeGen) gBase(b *BaseElem) {
	if !e.p.ok() {
		return
	}
	e.fuseHook()
	vname := b.Varname()
	if b.Convert {
		if b.ShimMode == Cast {
			vname = b.toBaseConvert()
		} else {
			vname = randIdent()
			e.p.declare(vname, b.BaseType())
			e.p.printf("\n%s, err = %s", vname, b.toBaseConvert())
			e.p.printf(errCheck)
		}
	}

	if b.Value == IDENT { // unknown identity
		e.p.printf("\nerr = %s.EncodeMsg(en)", vname)
		e.p.print(errCheck)
	} else { // typical case
		e.writeAndCheck(b.BaseName(), literalFmt, vname)
	}
}
