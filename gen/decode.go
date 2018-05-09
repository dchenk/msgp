package gen

import (
	"io"
	"strconv"
)

func decode(w io.Writer) *decodeGen {
	return &decodeGen{
		p: printer{w: w},
	}
}

type decodeGen struct {
	passes
	p        printer
	hasField bool
}

func (d *decodeGen) Method() Method { return Decode }

func (d *decodeGen) Execute(p Elem) error {
	p = d.applyAll(p)
	if p == nil {
		return nil
	}
	d.hasField = false
	if !d.p.ok() {
		return d.p.err
	}

	if !isPrintable(p) {
		return nil
	}

	d.p.comment("DecodeMsg implements msgp.Decoder")

	d.p.printf("\nfunc (%s %s) DecodeMsg(dc *msgp.Reader) (err error) {", p.Varname(), methodReceiver(p))
	next(d, p)
	d.p.nakedReturn()
	unsetReceiver(p)
	return d.p.err
}

func (d *decodeGen) gStruct(s *Struct) {
	if !d.p.ok() {
		return
	}
	if s.AsTuple {
		d.structAsTuple(s)
	} else {
		d.structAsMap(s)
	}
	return
}

func (d *decodeGen) assignAndCheck(name, typ string) {
	if !d.p.ok() {
		return
	}
	d.p.printf("\n%s, err = dc.Read%s()", name, typ)
	d.p.print(errCheck)
}

func (d *decodeGen) structAsTuple(s *Struct) {
	nfields := len(s.Fields)

	sz := randIdent()
	d.p.declare(sz, u32)
	d.assignAndCheck(sz, arrayHeader)
	d.p.arrayCheck(strconv.Itoa(nfields), sz)
	for i := range s.Fields {
		if !d.p.ok() {
			return
		}
		next(d, s.Fields[i].fieldElem)
	}
}

func (d *decodeGen) structAsMap(s *Struct) {

	if !d.hasField {
		d.p.declare("field", "[]byte")
		d.hasField = true
	}

	// Declare the variable that will contain the map length.
	sz := randIdent()
	d.p.declare(sz, u32)

	// Assign to the sz variable the length of the map.
	d.assignAndCheck(sz, mapHeader)

	d.p.printf("\nfor %s > 0 {", sz)
	d.p.printf("\n%s--", sz)
	d.assignAndCheck("field", mapKey)
	d.p.print("\nswitch string(field) {")
	for i := range s.Fields {
		d.p.printf("\ncase \"%s\":", s.Fields[i].fieldTag)
		next(d, s.Fields[i].fieldElem)
		if !d.p.ok() {
			return
		}
	}
	d.p.print("\ndefault:\nerr = dc.Skip()")
	d.p.print(errCheck)

	d.p.closeBlock() // close switch block
	d.p.closeBlock() // close for loop

}

func (d *decodeGen) gBase(b *BaseElem) {

	if !d.p.ok() {
		return
	}

	var tmp string
	if b.Convert {
		// Open 'tmp' block.
		d.p.print("\n{")
		tmp = randIdent()
		d.p.declare(tmp, b.BaseType())
	}

	vname := b.Varname()  // e.g. "z.FieldOne"
	bname := b.BaseName() // e.g. "Float64"

	// Handle special cases for object type.
	switch b.Value {
	case Bytes:
		if b.Convert {
			d.p.printf("\n%s, err = dc.ReadBytes([]byte(%s))", tmp, vname)
		} else {
			d.p.printf("\n%s, err = dc.ReadBytes(%s)", vname, vname)
		}
	case IDENT:
		d.p.printf("\nerr = %s.DecodeMsg(dc)", vname)
	case Ext:
		d.p.printf("\nerr = dc.ReadExtension(%s)", vname)
	default:
		if b.Convert {
			d.p.printf("\n%s, err = dc.Read%s()", tmp, bname)
		} else {
			d.p.printf("\n%s, err = dc.Read%s()", vname, bname)
		}
	}
	d.p.print(errCheck)

	if b.Convert {
		// Close 'tmp' block.
		if b.ShimMode == Cast {
			d.p.printf("\n%s = %s(%s)\n}", vname, b.FromBase(), tmp)
		} else {
			d.p.printf("\n%s, err = %s(%s)\n}", vname, b.FromBase(), tmp)
			d.p.print(errCheck)
		}
	}

}

func (d *decodeGen) gMap(m *Map) {
	if !d.p.ok() {
		return
	}
	sz := randIdent()

	// resize or allocate map
	d.p.declare(sz, u32)
	d.assignAndCheck(sz, mapHeader)
	d.p.resizeMap(sz, m)

	// for element in map, read string/value
	// pair and assign
	d.p.printf("\nfor %s > 0 {\n%s--", sz, sz)
	d.p.declare(m.KeyIndx, "string")
	d.p.declare(m.ValIndx, m.Value.TypeName())
	d.assignAndCheck(m.KeyIndx, stringTyp)
	next(d, m.Value)
	d.p.mapAssign(m)
	d.p.closeBlock()
}

func (d *decodeGen) gSlice(s *Slice) {
	if !d.p.ok() {
		return
	}
	sz := randIdent()
	d.p.declare(sz, u32)
	d.assignAndCheck(sz, arrayHeader)
	d.p.resizeSlice(sz, s)
	d.p.rangeBlock(s.Index, s.Varname(), d, s.Els)
}

func (d *decodeGen) gArray(a *Array) {
	if !d.p.ok() {
		return
	}

	// special case if we have [const]byte
	if be, ok := a.Els.(*BaseElem); ok && (be.Value == Byte || be.Value == Uint8) {
		d.p.printf("\nerr = dc.ReadExactBytes((%s)[:])", a.Varname())
		d.p.print(errCheck)
		return
	}
	sz := randIdent()
	d.p.declare(sz, u32)
	d.assignAndCheck(sz, arrayHeader)
	d.p.arrayCheck(coerceArraySize(a.Size), sz)

	d.p.rangeBlock(a.Index, a.Varname(), d, a.Els)
}

func (d *decodeGen) gPtr(p *Ptr) {
	if !d.p.ok() {
		return
	}
	d.p.print("\nif dc.IsNil() {")
	d.p.print("\nerr = dc.ReadNil()")
	d.p.print(errCheck)
	d.p.printf("\n%s = nil\n} else {", p.Varname())
	d.p.initPtr(p)
	next(d, p.Value)
	d.p.closeBlock()
}
