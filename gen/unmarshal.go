package gen

import (
	"io"
	"strconv"
)

func unmarshal(w io.Writer) *unmarshalGen {
	return &unmarshalGen{
		p: printer{w: w},
	}
}

type unmarshalGen struct {
	passes
	p        printer
	hasField bool
}

func (u *unmarshalGen) Method() Method { return Unmarshal }

func (u *unmarshalGen) Execute(p Elem) error {

	u.hasField = false
	if !u.p.ok() {
		return u.p.err
	}
	p = u.applyAll(p)
	if p == nil {
		return nil
	}
	if !isPrintable(p) {
		return nil
	}

	u.p.comment("UnmarshalMsg implements msgp.Unmarshaler")

	u.p.printf("\nfunc (%s %s) UnmarshalMsg(bts []byte) (o []byte, err error) {", p.Varname(), methodReceiver(p))
	next(u, p)
	u.p.print("\no = bts")
	u.p.nakedReturn()
	unsetReceiver(p)
	return u.p.err

}

// does assignment to the variable "name" with the type "base"
func (u *unmarshalGen) assignAndCheck(name, base string) {
	if !u.p.ok() {
		return
	}
	u.p.printf("\n%s, bts, err = msgp.Read%sBytes(bts)", name, base)
	u.p.print(errCheck)
}

func (u *unmarshalGen) gStruct(s *Struct) {
	if !u.p.ok() {
		return
	}
	if s.AsTuple {
		u.tuple(s)
	} else {
		u.structAsMap(s)
	}
	return
}

func (u *unmarshalGen) tuple(s *Struct) {
	sz := randIdent()
	u.p.declare(sz, u32)
	u.assignAndCheck(sz, arrayHeader)
	u.p.arrayCheck(strconv.Itoa(len(s.Fields)), sz)
	for i := range s.Fields {
		if !u.p.ok() {
			return
		}
		next(u, s.Fields[i].fieldElem)
	}
}

func (u *unmarshalGen) structAsMap(s *Struct) {

	if !u.hasField {
		u.p.declare("field", "[]byte")
		u.hasField = true
	}

	// Declare the variable that will contain the map length.
	sz := randIdent()
	u.p.declare(sz, u32)

	// Assign to the sz variable the length of the map, and get remaining bytes
	// in a variable named "bts".
	u.assignAndCheck(sz, mapHeader)

	u.p.printf("\nfor %s > 0 {", sz)
	u.p.printf("\n%s--", sz)
	u.p.print("\nfield, bts, err = msgp.ReadMapKeyZC(bts)")
	u.p.print(errCheck)
	u.p.print("\nswitch string(field) {")
	for i := range s.Fields {
		if !u.p.ok() {
			return
		}
		u.p.printf("\ncase \"%s\":", s.Fields[i].fieldTag)
		next(u, s.Fields[i].fieldElem)
	}
	u.p.print("\ndefault:\nbts, err = msgp.Skip(bts)")
	u.p.print(errCheck)

	u.p.closeBlock() // close switch block
	u.p.closeBlock() // close for loop

}

func (u *unmarshalGen) gBase(b *BaseElem) {

	if !u.p.ok() {
		return
	}

	refname := b.Varname() // assigned to
	lowered := b.Varname() // passed as argument

	if b.Convert {
		// Open 'tmp' block.
		lowered = b.ToBase() + "(" + lowered + ")"
		u.p.print("\n{") // inner scope
		refname = randIdent()
		u.p.declare(refname, b.BaseType())
	}

	switch b.Value {
	case Bytes:
		u.p.printf("\n%s, bts, err = msgp.ReadBytesBytes(bts, %s)", refname, lowered)
	case Ext:
		u.p.printf("\nbts, err = msgp.ReadExtensionBytes(bts, %s)", lowered)
	case IDENT:
		u.p.printf("\nbts, err = %s.UnmarshalMsg(bts)", lowered)
	default:
		u.p.printf("\n%s, bts, err = msgp.Read%sBytes(bts)", refname, b.BaseName())
	}
	u.p.print(errCheck)

	if b.Convert {
		// Close 'tmp' block.
		if b.ShimMode == Cast {
			u.p.printf("\n%s = %s(%s)\n", b.Varname(), b.FromBase(), refname)
		} else {
			u.p.printf("\n%s, err = %s(%s)", b.Varname(), b.FromBase(), refname)
			u.p.print(errCheck)
		}
		u.p.printf("}")
	}

}

func (u *unmarshalGen) gArray(a *Array) {
	if !u.p.ok() {
		return
	}

	// special case for [const]byte objects
	// see decode.go for symmetry
	if be, ok := a.Els.(*BaseElem); ok && be.Value == Byte {
		u.p.printf("\nbts, err = msgp.ReadExactBytes(bts, (%s)[:])", a.Varname())
		u.p.print(errCheck)
		return
	}

	sz := randIdent()
	u.p.declare(sz, u32)
	u.assignAndCheck(sz, arrayHeader)
	u.p.arrayCheck(coerceArraySize(a.Size), sz)
	u.p.rangeBlock(a.Index, a.Varname(), u, a.Els)
}

func (u *unmarshalGen) gSlice(s *Slice) {
	if !u.p.ok() {
		return
	}
	sz := randIdent()
	u.p.declare(sz, u32)
	u.assignAndCheck(sz, arrayHeader)
	u.p.resizeSlice(sz, s)
	u.p.rangeBlock(s.Index, s.Varname(), u, s.Els)
}

func (u *unmarshalGen) gMap(m *Map) {
	if !u.p.ok() {
		return
	}
	sz := randIdent()
	u.p.declare(sz, u32)
	u.assignAndCheck(sz, mapHeader)

	// Allocate or clear map
	u.p.resizeMap(sz, m)

	// Loop and get key, value
	u.p.printf("\nfor %s > 0 {", sz)
	u.p.declare(m.KeyIndx, "string")
	u.p.declare(m.ValIndx, m.Value.TypeName())
	u.p.printf("\n%s--", sz)
	u.assignAndCheck(m.KeyIndx, stringTyp)
	next(u, m.Value)
	u.p.mapAssign(m)
	u.p.closeBlock()
}

func (u *unmarshalGen) gPtr(p *Ptr) {
	u.p.printf("\nif msgp.IsNil(bts) { bts, err = msgp.ReadNilBytes(bts); if err != nil { return }; %s = nil; } else { ", p.Varname())
	u.p.initPtr(p)
	next(u, p.Value)
	u.p.closeBlock()
}
