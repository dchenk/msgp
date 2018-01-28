package gen

import (
	"fmt"
	"strings"
)

var (
	identNext   = 0
	identPrefix = "za"
)

func resetIdent(prefix string) {
	identPrefix = prefix
	identNext = 0
}

// randIdent generates a random identifier name.
func randIdent() string {
	identNext++
	return fmt.Sprintf("%s%04d", identPrefix, identNext)
}

// This code defines the type declaration tree.
//
// Consider the following:
//
//  type Marshaler struct {
//  	Thing1 *float64 `msg:"thing1"`
// 		Body   []byte   `msg:"body"`
//  }
//
// A parser should parse the above into:
//
//  var val Elem = &Ptr{
// 		name: "z",
// 		Value: &Struct{
// 			Name: "Marshaler",
// 			Fields: []structField{
// 				{
// 					fieldTag: "thing1",
// 					fieldElem: &Ptr{
// 						name: "z.Thing1",
// 						Value: &BaseElem{
// 							name:    "*z.Thing1",
// 							Value:   Float64,
//							Convert: false,
// 						},
// 					},
// 				},
// 				{
// 					fieldTag: "body",
// 						fieldElem: &BaseElem{
// 						name:    "z.Body",
// 						Value:   Bytes,
// 						Convert: false,
// 					},
// 				},
// 			},
// 		},
//  }

// A primitive is a basic type to msgp.
type primitive uint8

// This list of primitive types is effectively the list of types
// currently having ReadXxxx and WriteXxxx methods.
const (
	Invalid primitive = iota
	Bytes
	String
	Float32
	Float64
	Complex64
	Complex128
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Byte
	Int
	Int8
	Int16
	Int32
	Int64
	Bool
	Intf // interface{}
	Time // time.Time
	Ext  // extension

	IDENT // IDENT means an unrecognized identifier
)

func (k primitive) String() string {
	switch k {
	case String:
		return "String"
	case Bytes:
		return "Bytes"
	case Float32:
		return "Float32"
	case Float64:
		return "Float64"
	case Complex64:
		return "Complex64"
	case Complex128:
		return "Complex128"
	case Uint:
		return "Uint"
	case Uint8:
		return "Uint8"
	case Uint16:
		return "Uint16"
	case Uint32:
		return "Uint32"
	case Uint64:
		return "Uint64"
	case Byte:
		return "Byte"
	case Int:
		return "Int"
	case Int8:
		return "Int8"
	case Int16:
		return "Int16"
	case Int32:
		return "Int32"
	case Int64:
		return "Int64"
	case Bool:
		return "Bool"
	case Intf:
		return "Intf"
	case Time:
		return "time.Time"
	case Ext:
		return "Extension"
	case IDENT:
		return "Ident"
	default:
		return "INVALID"
	}
}

// primitives lists all of the recognized identities that have
// a corresponding primitive type.
var primitives = map[string]primitive{
	"[]byte":         Bytes,
	"string":         String,
	"float32":        Float32,
	"float64":        Float64,
	"complex64":      Complex64,
	"complex128":     Complex128,
	"uint":           Uint,
	"uint8":          Uint8,
	"uint16":         Uint16,
	"uint32":         Uint32,
	"uint64":         Uint64,
	"byte":           Byte,
	"rune":           Int32,
	"int":            Int,
	"int8":           Int8,
	"int16":          Int16,
	"int32":          Int32,
	"int64":          Int64,
	"bool":           Bool,
	"interface{}":    Intf,
	"time.Time":      Time,
	"msgp.Extension": Ext,
}

// builtIns are types built into the library
// that satisfy all of the interfaces.
var builtIns = map[string]struct{}{
	"msgp.Raw":    struct{}{},
	"msgp.Number": struct{}{},
}

// common data/methods for every Elem
type common struct{ vname, alias string }

func (c *common) SetVarname(s string) { c.vname = s }
func (c *common) Varname() string     { return c.vname }
func (c *common) Alias(typ string)    { c.alias = typ }

func isPrintable(e Elem) bool {
	if be, ok := e.(*BaseElem); ok && !be.Printable() {
		return false
	}
	return true
}

// An Elem is a Go type capable of being serialized into MessagePack.
// It is implemented by *Ptr, *Struct, *Array, *Slice, *Map, and *BaseElem.
type Elem interface {
	// SetVarname sets this node's variable name and recursively
	// sets the names of all its children. In general, this
	// should only be called on the parent of the tree.
	SetVarname(s string)

	// Varname returns the variable name of the element.
	Varname() string

	// TypeName is the canonical Go type name of the node, such as "string",
	// "int", "map[string]float64" OR the alias name, if it has been set.
	TypeName() string

	// Alias sets a type (alias) name.
	Alias(typ string)

	// Copy performs a deep copy of the object.
	Copy() Elem

	// Complexity returns a measure of the complexity of the element (greater
	// than or equal to 1).
	Complexity() int
}

// Ident returns the *BaseElem that corresponds to the provided identity.
func Ident(id string) *BaseElem {
	p, ok := primitives[id]
	if ok {
		return &BaseElem{Value: p}
	}
	be := &BaseElem{Value: IDENT}
	be.Alias(id)
	return be
}

type Array struct {
	common
	Index string // index variable name
	Size  string // array size
	Els   Elem   // child
}

func (a *Array) SetVarname(s string) {

	a.common.SetVarname(s)
ridx:
	a.Index = randIdent()

	// Try to avoid using the same index as a parent slice.
	if strings.Contains(a.Varname(), a.Index) {
		goto ridx
	}

	a.Els.SetVarname(fmt.Sprintf("%s[%s]", a.Varname(), a.Index))

}

func (a *Array) TypeName() string {
	if a.common.alias != "" {
		return a.common.alias
	}
	a.common.Alias(fmt.Sprintf("[%s]%s", a.Size, a.Els.TypeName()))
	return a.common.alias
}

func (a *Array) Copy() Elem {
	b := *a
	b.Els = a.Els.Copy()
	return &b
}

func (a *Array) Complexity() int { return 1 + a.Els.Complexity() }

// Map is a map[string]Elem
type Map struct {
	common
	Keyidx string // key variable name
	Validx string // value variable name
	Value  Elem   // value element
}

func (m *Map) SetVarname(s string) {
	m.common.SetVarname(s)
ridx:
	m.Keyidx = randIdent()
	m.Validx = randIdent()

	// just in case
	if m.Keyidx == m.Validx {
		goto ridx
	}

	m.Value.SetVarname(m.Validx)
}

func (m *Map) TypeName() string {
	if m.common.alias != "" {
		return m.common.alias
	}
	m.common.Alias("map[string]" + m.Value.TypeName())
	return m.common.alias
}

func (m *Map) Copy() Elem {
	g := *m
	g.Value = m.Value.Copy()
	return &g
}

func (m *Map) Complexity() int { return 2 + m.Value.Complexity() }

type Slice struct {
	common
	Index string
	Els   Elem // The type of each element
}

func (s *Slice) SetVarname(a string) {
	s.common.SetVarname(a)
	s.Index = randIdent()
	varName := s.Varname()
	if varName[0] == '*' {
		// Pointer-to-slice requires parenthesis for slicing.
		varName = "(" + varName + ")"
	}
	s.Els.SetVarname(fmt.Sprintf("%s[%s]", varName, s.Index))
}

func (s *Slice) TypeName() string {
	if s.common.alias != "" {
		return s.common.alias
	}
	s.common.Alias("[]" + s.Els.TypeName())
	return s.common.alias
}

func (s *Slice) Copy() Elem {
	z := *s
	z.Els = s.Els.Copy()
	return &z
}

func (s *Slice) Complexity() int {
	return 1 + s.Els.Complexity()
}

type Ptr struct {
	common
	Value Elem
}

func (s *Ptr) SetVarname(a string) {
	s.common.SetVarname(a)

	// struct fields are dereferenced
	// automatically...
	switch x := s.Value.(type) {
	case *Struct:
		// struct fields are automatically dereferenced
		x.SetVarname(a)
		return

	case *BaseElem:
		// identities have pointer receivers
		if x.Value == IDENT {
			x.SetVarname(a)
		} else {
			x.SetVarname("*" + a)
		}
		return

	default:
		s.Value.SetVarname("*" + a)
		return
	}
}

func (s *Ptr) TypeName() string {
	if s.common.alias != "" {
		return s.common.alias
	}
	s.common.Alias("*" + s.Value.TypeName())
	return s.common.alias
}

func (s *Ptr) Copy() Elem {
	v := *s
	v.Value = s.Value.Copy()
	return &v
}

func (s *Ptr) Complexity() int { return 1 + s.Value.Complexity() }

func (s *Ptr) Needsinit() bool {
	if be, ok := s.Value.(*BaseElem); ok && be.needsref {
		return false
	}
	return true
}

type Struct struct {
	common
	Fields  []structField // field list
	AsTuple bool          // write as an array instead of a map
}

func (s *Struct) TypeName() string {
	if s.common.alias != "" {
		return s.common.alias
	}
	str := "struct{\n"
	for i := range s.Fields {
		str += s.Fields[i].fieldName +
			" " + s.Fields[i].fieldElem.TypeName() +
			" " + s.Fields[i].rawTag + "\n"
	}
	str += "}"
	s.common.Alias(str)
	return s.common.alias
}

func (s *Struct) SetVarname(a string) {
	s.common.SetVarname(a)
	writeStructFields(s.Fields, a)
}

func (s *Struct) Copy() Elem {
	g := *s
	g.Fields = make([]structField, len(s.Fields))
	copy(g.Fields, s.Fields)
	for i := range s.Fields {
		g.Fields[i].fieldElem = s.Fields[i].fieldElem.Copy()
	}
	return &g
}

func (s *Struct) Complexity() int {
	c := 1
	for i := range s.Fields {
		c += s.Fields[i].fieldElem.Complexity()
	}
	return c
}

type structField struct {
	fieldTag  string // the string inside the `msg:""` tag
	rawTag    string // the full tag (in case there are non-msg keys)
	fieldName string // the name of the struct field
	fieldElem Elem   // the field type
}

// writeStructFields is a trampoline for writeBase for all of the fields in a struct.
func writeStructFields(s []structField, structName string) {
	for i := range s {
		s[i].fieldElem.SetVarname(fmt.Sprintf("%s.%s", structName, s[i].fieldName))
	}
}

type ShimMode int

const (
	Cast ShimMode = iota
	Convert
)

// A BaseElem is an element that can be represented by a primitive MessagePack type.
type BaseElem struct {
	common
	ShimMode     ShimMode  // Method used to shim
	ShimToBase   string    // shim to base type, or empty
	ShimFromBase string    // shim from base type, or empty
	Value        primitive // Type of element
	Convert      bool      // should we do an explicit conversion?
	mustinline   bool      // must inline; not printable
	needsref     bool      // needs reference for shim
}

func (s *BaseElem) Printable() bool { return !s.mustinline }

func (s *BaseElem) Alias(typ string) {
	s.common.Alias(typ)
	if s.Value != IDENT {
		s.Convert = true
	}
	if strings.Contains(typ, ".") {
		s.mustinline = true
	}
}

func (s *BaseElem) SetVarname(a string) {
	// Ext types whose parents are not pointers need
	// to be explicitly referenced.
	if s.Value == Ext || s.needsref {
		if strings.HasPrefix(a, "*") {
			s.common.SetVarname(a[1:])
			return
		}
		s.common.SetVarname("&" + a)
		return
	}

	s.common.SetVarname(a)
}

// TypeName returns the syntactically correct Go type name for the base element.
func (s *BaseElem) TypeName() string {
	if s.common.alias != "" {
		return s.common.alias
	}
	s.common.Alias(s.BaseType())
	return s.common.alias
}

// ToBase, used if Convert==true, is used as tmp = {{ToBase}}({{Varname}})
func (b *BaseElem) ToBase() string {
	if b.ShimToBase != "" {
		return b.ShimToBase
	}
	return b.BaseType()
}

// FromBase, used if Convert==true, is used as {{Varname}} = {{FromBase}}(tmp)
func (b *BaseElem) FromBase() string {
	if b.ShimFromBase != "" {
		return b.ShimFromBase
	}
	return b.TypeName()
}

func (b *BaseElem) toBaseConvert() string {
	return b.ToBase() + "(" + b.Varname() + ")"
}

// BaseName returns the string form of the
// base type (e.g. Float64, Ident, etc)
func (s *BaseElem) BaseName() string {
	// time is a special case;
	// we strip the package prefix
	if s.Value == Time {
		return "Time"
	}
	return s.Value.String()
}

func (s *BaseElem) BaseType() string {
	switch s.Value {
	case IDENT:
		return s.TypeName()

	// exceptions to the naming/capitalization
	// rule:
	case Intf:
		return "interface{}"
	case Bytes:
		return "[]byte"
	case Time:
		return "time.Time"
	case Ext:
		return "msgp.Extension"

	// everything else is base.String() with
	// the first letter as lowercase
	default:
		return strings.ToLower(s.BaseName())
	}
}

func (s *BaseElem) Needsref(b bool) {
	s.needsref = b
}

func (s *BaseElem) Copy() Elem {
	g := *s
	return &g
}

func (s *BaseElem) Complexity() int {
	if s.Convert && !s.mustinline {
		return 2
	}
	// we need to return 1 if !printable(),
	// in order to make sure that stuff gets
	// inlined appropriately
	return 1
}

// Resolved says whether or not the type of the element is a primitive
// or a builtin provided by the package.
func (s *BaseElem) Resolved() bool {
	if s.Value == IDENT {
		_, ok := builtIns[s.TypeName()]
		return ok
	}
	return true
}

// coerceArraySize ensures we can compare constant array lengths.
// MessagePack array headers are (up to) 32 bit unsigned, which is reflected in the
// ArrayHeader implementation in this library using uint32. On the Go side, we can
// declare array lengths as any constant integer width, which breaks when attempting
// a direct comparison to an array header's uint32.
func coerceArraySize(asz string) string {
	return fmt.Sprintf("uint32(%s)", asz)
}
