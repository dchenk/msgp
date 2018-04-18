package gen

import (
	"fmt"
	"io"
	"regexp"
)

const (
	errCheck    = "\nif err != nil { return }"
	lenAsUint32 = "uint32(len(%s))"
	literalFmt  = "%s"
	mapHeader   = "MapHeader"
	arrayHeader = "ArrayHeader"
	mapKey      = "MapKeyPtr"
	stringTyp   = "String"
	u32         = "uint32"
)

// A Method is a bitfield representing something that the
// generator knows how to print.
type Method uint8

// isSet says if the bits in 'f' are set in 'm'
func (m Method) isSet(f Method) bool { return m&f == f }

// String implements fmt.Stringer
func (m Method) String() string {
	switch m {
	case 0, invalidmeth:
		return "<invalid method>"
	case Decode:
		return "decode"
	case Encode:
		return "encode"
	case Marshal:
		return "marshal"
	case Unmarshal:
		return "unmarshal"
	case Size:
		return "size"
	case Test:
		return "test"
	default:
		// return something like "decode+encode+test"
		modes := [...]Method{Decode, Encode, Marshal, Unmarshal, Size, Test}
		any := false
		nm := ""
		for _, mm := range modes {
			if m.isSet(mm) {
				if any {
					nm += "+" + mm.String()
				} else {
					nm += mm.String()
					any = true
				}
			}
		}
		return nm
	}
}

// The following methods indicate for each pass what interfaces types should implement.
const (
	Decode      Method                       = 1 << iota // Decode using msgp.Decoder
	Encode                                               // Encode using msgp.Encoder
	Marshal                                              // Marshal using msgp.Marshaler
	Unmarshal                                            // Unmarshal using msgp.Unmarshaler
	Size                                                 // Size using msgp.Sizer
	Test                                                 // Test functions should be generated
	invalidmeth                                          // this isn't a method
	encodetest  = Encode | Decode | Test                 // tests for Encoder and Decoder
	marshaltest = Marshal | Unmarshal | Test             // tests for Marshaler and Unmarshaler
)

// A generator has all the methods needed to generate code.
type generator interface {
	Method() Method
	Add(p TransformPass)
	Execute(Elem) error
}

type generatorSet []generator

func newGeneratorSet(m Method, out io.Writer, tests io.Writer) generatorSet {
	if m.isSet(Test) && tests == nil {
		panic("cannot print tests with 'nil' tests argument")
	}
	gens := make(generatorSet, 0, 7)
	if m.isSet(Decode) {
		gens = append(gens, decode(out))
	}
	if m.isSet(Encode) {
		gens = append(gens, encode(out))
	}
	if m.isSet(Marshal) {
		gens = append(gens, marshal(out))
	}
	if m.isSet(Unmarshal) {
		gens = append(gens, unmarshal(out))
	}
	if m.isSet(Size) {
		gens = append(gens, sizes(out))
	}
	if m.isSet(marshaltest) {
		gens = append(gens, mtest(tests))
	}
	if m.isSet(encodetest) {
		gens = append(gens, etest(tests))
	}
	if len(gens) == 0 {
		panic("newGeneratorSet called with invalid method flags")
	}
	return gens
}

// ApplyDirective applies a directive to a named pass and all of its dependents.
func (gs generatorSet) ApplyDirective(pass Method, t TransformPass) {
	for _, g := range gs {
		if g.Method().isSet(pass) {
			g.Add(t)
		}
	}
}

// Print prints an Elem.
func (gs generatorSet) Print(e Elem) error {
	for _, g := range gs {
		// Elem.SetVarname() is called before the Print() step in parse.FileSet.PrintTo().
		// Elem.SetVarname() generates identifiers as it walks the Elem. This can cause
		// collisions between idents created during SetVarname and idents created during Print,
		// hence the separate prefixes.
		resetIdent("zb")
		err := g.Execute(e)
		resetIdent("za")
		if err != nil {
			return err
		}
	}
	return nil
}

// A TransformPass is a pass that transforms individual elements. If the returned is
// different from the argument, it should not point to the same objects.
type TransformPass func(Elem) Elem

// IgnoreTypename is a pass that just ignores types of a given name.
func IgnoreTypename(pattern string) TransformPass {
	return func(e Elem) Elem {
		if typeNameMatches(pattern, e.TypeName()) {
			return nil
		}
		return e
	}
}

// typeNameMatches compares the pattern to the typeName to see if the type name
// satisfies the pattern. The pattern can be either simply the type name itself
// or a regexp pattern. A regexp pattern is extracted by taking everything that
// follows either "reg=" or "reg!=" in the pattern string.
// Pattern "reg=expr" returns true if and only if typeName matches expr.
// Pattern "reg!=expr" returns true if and only if typeName does NOT match expr.
func typeNameMatches(pattern, typeName string) bool {
	if len(pattern) > 4 && pattern[:3] == "reg" {
		if string(pattern[3]) == "!" {
			if !regexp.MustCompile(pattern[5:]).MatchString(typeName) {
				fmt.Printf("Matched negated regexp %q to type %q\n", pattern[5:], typeName)
				return true
			}
		} else if regexp.MustCompile(pattern[4:]).MatchString(typeName) {
			fmt.Printf("Matched regexp %q to type %q\n", pattern[4:], typeName)
			return true
		}
		return false
	}
	// Concrete type names compare by simple equality.
	return pattern == typeName
}

type passes []TransformPass

func (p *passes) Add(t TransformPass) {
	*p = append(*p, t)
}

func (p *passes) applyall(e Elem) Elem {
	for _, t := range *p {
		e = t(e) // Execute the TransformPass func.
		if e == nil {
			return nil
		}
	}
	return e
}

type traversal interface {
	gMap(*Map)
	gSlice(*Slice)
	gArray(*Array)
	gPtr(*Ptr)
	gBase(*BaseElem)
	gStruct(*Struct)
}

// Call the method corresponding to the type of Elem e.
func next(t traversal, e Elem) {
	switch e := e.(type) {
	case *Map:
		t.gMap(e)
	case *Struct:
		t.gStruct(e)
	case *Slice:
		t.gSlice(e)
	case *Array:
		t.gArray(e)
	case *Ptr:
		t.gPtr(e)
	case *BaseElem:
		t.gBase(e)
	default:
		panic("bad element type")
	}
}

// possibly-immutable method receiver
func imutMethodReceiver(p Elem) string {
	switch e := p.(type) {
	case *Struct:
		// TODO(HACK): actually do real math here.
		if len(e.Fields) <= 3 {
			for i := range e.Fields {
				if be, ok := e.Fields[i].fieldElem.(*BaseElem); !ok || (be.Value == IDENT || be.Value == Bytes) {
					goto nope
				}
			}
			return p.TypeName()
		}
	nope:
		return "*" + p.TypeName()

	// Arrays get de-referenced automatically.
	case *Array:
		return "*" + p.TypeName()

	// Everything else can be by-value.
	default:
		return p.TypeName()
	}
}

// methodReceiver wraps, if necessary, a type so that
// its method receiver is of the right type.
func methodReceiver(p Elem) string {
	switch p.(type) {
	// structs and arrays are de-referenced
	// automatically, so no need to alter varname
	case *Struct, *Array:
		return "*" + p.TypeName()
	// set variable name to *varname
	default:
		p.SetVarname("(*" + p.Varname() + ")")
		return "*" + p.TypeName()
	}
}

// unsetReceiver sets Varname to "z" for the Elem if its not a *Struct or *Array.
func unsetReceiver(e Elem) {
	switch e.(type) {
	case *Struct, *Array:
	default:
		e.SetVarname("z")
	}
}

// The printer type is a shared utility for generators.
type printer struct {
	w   io.Writer
	err error
}

// writes "var {{name}} {{typ}}"
func (p *printer) declare(name string, typ string) {
	p.printf("\nvar %s %s", name, typ)
}

// resizeMap does:
//
// if m != nil && size > 0 {
//     m = make(type, size)
// } else if len(m) > 0 {
//     for key := range m { delete(m, key) }
// }
//
func (p *printer) resizeMap(size string, m *Map) {
	vn := m.Varname()
	if !p.ok() {
		return
	}
	p.printf("\nif %s == nil && %s > 0 {", vn, size)
	p.printf("\n%s = make(%s, %s)", vn, m.TypeName(), size)
	p.printf("\n} else if len(%s) > 0 {", vn)
	p.clearMap(vn)
	p.closeBlock()
}

// assign key to value based on varnames
func (p *printer) mapAssign(m *Map) {
	if p.ok() {
		p.printf("\n%s[%s] = %s", m.Varname(), m.KeyIndx, m.ValIndx)
	}
}

// clear map keys
func (p *printer) clearMap(name string) {
	p.printf("\nfor key := range %[1]s { delete(%[1]s, key) }", name)
}

func (p *printer) resizeSlice(size string, s *Slice) {
	p.printf("\nif cap(%[1]s) >= int(%[2]s) { %[1]s = (%[1]s)[:%[2]s] } else { %[1]s = make(%[3]s, %[2]s) }", s.Varname(), size, s.TypeName())
}

func (p *printer) arrayCheck(want, got string) {
	p.printf("\nif %[1]s != %[2]s { err = msgp.ArrayError{Wanted: %[2]s, Got: %[1]s}; return }", got, want)
}

// rangeBlock prints:
//  for idx := range iter {
//  	{{generate inner}}
//  }
func (p *printer) rangeBlock(idx string, iter string, t traversal, inner Elem) {
	p.printf("\n for %s := range %s {", idx, iter)
	next(t, inner)
	p.closeBlock()
}

func (p *printer) closeBlock() {
	p.print("\n}")
}

func (p *printer) nakedReturn() {
	if p.ok() {
		p.print("\nreturn\n}\n")
	}
}

func (p *printer) comment(s string) {
	p.print("\n// " + s)
}

func (p *printer) printf(format string, args ...interface{}) {
	if p.ok() {
		_, p.err = fmt.Fprintf(p.w, format, args...)
	}
}

func (p *printer) print(format string) {
	if p.ok() {
		_, p.err = io.WriteString(p.w, format)
	}
}

func (p *printer) initPtr(pt *Ptr) {
	if pt.NeedsInit() {
		vn := pt.Varname()
		p.printf("\nif %s == nil {\n%s = new(%s)\n}", vn, vn, pt.Value.TypeName())
	}
}

func (p *printer) ok() bool { return p.err == nil }
