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
//  	Thing1 *float64 `msgp:"thing1"`
// 		Body   []byte   `msgp:"body"`
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

// String implements io.Stringer for primitive.
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
	"msgp.Raw":    {},
	"msgp.Number": {},
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
	// SetVarname sets the node's variable name and recursively the names of
	// all its children. In general, this should only be called on the parent
	// of the tree.
	SetVarname(s string)

	// Varname returns the variable name of the element.
	Varname() string

	// TypeName is the canonical Go type name of the node, such as "string",
	// "int", "map[string]float64" OR the alias name, if it has been set.
	TypeName() string

	// Alias sets a type (alias) name.
	Alias(typ string)

	// Copy returns a deep copy of the object.
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

// Array represents an array.
type Array struct {
	common
	Index string // index variable name
	Size  string // array size
	Els   Elem   // child
}

// SetVarname sets the name of the array and its index variable.
func (a *Array) SetVarname(s string) {
	a.common.SetVarname(s)
	// Avoid using the same index as a parent slice.
	for a.Index == "" || strings.Contains(a.Varname(), a.Index) {
		a.Index = randIdent()
	}
	a.Els.SetVarname(a.Varname() + "[" + a.Index + "]")
}

// TypeName returns the canonical Go type name.
func (a *Array) TypeName() string {
	if a.common.alias != "" {
		return a.common.alias
	}
	a.common.Alias("[" + a.Size + "]" + a.Els.TypeName())
	return a.common.alias
}

// Copy returns a deep copy of the object.
func (a *Array) Copy() Elem {
	b := *a
	b.Els = a.Els.Copy()
	return &b
}

// Complexity returns a measure of the complexity of the element.
func (a *Array) Complexity() int { return 1 + a.Els.Complexity() }

// Map is a map[string]Elem.
type Map struct {
	common
	KeyIndx string // key variable name
	ValIndx string // value variable name
	Value   Elem   // value element
}

// SetVarname sets the names of the map and the index variables.
func (m *Map) SetVarname(s string) {
	m.common.SetVarname(s)
	m.KeyIndx = randIdent()
	for m.ValIndx == "" || m.ValIndx == m.KeyIndx {
		m.ValIndx = randIdent()
	}
	m.Value.SetVarname(m.ValIndx)
}

// TypeName returns the canonical Go type name.
func (m *Map) TypeName() string {
	if m.common.alias != "" {
		return m.common.alias
	}
	m.common.Alias("map[string]" + m.Value.TypeName())
	return m.common.alias
}

// Copy returns a deep copy of the object.
func (m *Map) Copy() Elem {
	g := *m
	g.Value = m.Value.Copy()
	return &g
}

// Complexity returns a measure of the complexity of the element.
func (m *Map) Complexity() int { return 2 + m.Value.Complexity() }

// Slice represents a slice.
type Slice struct {
	common
	Index string
	Els   Elem // The type of each element
}

// SetVarname sets the name of the slice and its index variable.
func (s *Slice) SetVarname(n string) {
	s.common.SetVarname(n)
	s.Index = randIdent()
	vn := s.Varname()
	if vn[0] == '*' {
		// Pointer-to-slice requires parenthesis for slicing.
		vn = "(" + vn + ")"
	}
	s.Els.SetVarname(vn + "[" + s.Index + "]")
}

// TypeName returns the canonical Go type name.
func (s *Slice) TypeName() string {
	if s.common.alias != "" {
		return s.common.alias
	}
	s.common.Alias("[]" + s.Els.TypeName())
	return s.common.alias
}

// Copy returns a deep copy of the object.
func (s *Slice) Copy() Elem {
	z := *s
	z.Els = s.Els.Copy()
	return &z
}

// Complexity returns a measure of the complexity of the element.
func (s *Slice) Complexity() int {
	return 1 + s.Els.Complexity()
}

// Ptr represents a pointer.
type Ptr struct {
	common
	Value Elem
}

// SetVarname sets the name of the pointer variable.
func (s *Ptr) SetVarname(n string) {
	s.common.SetVarname(n)
	switch x := s.Value.(type) {
	case *Struct:
		// Struct fields are de-referenced automatically.
		x.SetVarname(n)
		return
	case *BaseElem:
		// identities have pointer receivers
		if x.Value == IDENT {
			x.SetVarname(n)
		} else {
			x.SetVarname("*" + n)
		}
		return
	default:
		s.Value.SetVarname("*" + n)
		return
	}
}

// TypeName returns the canonical Go type name.
func (s *Ptr) TypeName() string {
	if s.common.alias != "" {
		return s.common.alias
	}
	s.common.Alias("*" + s.Value.TypeName())
	return s.common.alias
}

// Copy returns a deep copy of the object.
func (s *Ptr) Copy() Elem {
	v := *s
	v.Value = s.Value.Copy()
	return &v
}

// Complexity returns a measure of the complexity of the element.
func (s *Ptr) Complexity() int { return 1 + s.Value.Complexity() }

// NeedsInit says if the pointer needs to be checked if it should be newly allocated for use.
func (s *Ptr) NeedsInit() bool {
	if be, ok := s.Value.(*BaseElem); ok && be.needsref {
		return false
	}
	return true
}

// Struct represents a struct.
type Struct struct {
	common
	Fields  []structField // field list
	AsTuple bool          // write as an array instead of a map
}

// TypeName returns the canonical Go type name.
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

// SetVarname sets the name of the struct variable.
func (s *Struct) SetVarname(a string) {
	s.common.SetVarname(a)
	writeStructFields(s.Fields, a)
}

// Copy returns a deep copy of the object.
func (s *Struct) Copy() Elem {
	g := *s
	g.Fields = make([]structField, len(s.Fields))
	copy(g.Fields, s.Fields)
	for i := range s.Fields {
		g.Fields[i].fieldElem = s.Fields[i].fieldElem.Copy()
	}
	return &g
}

// Complexity returns a measure of the complexity of the element.
func (s *Struct) Complexity() int {
	c := 1
	for i := range s.Fields {
		c += s.Fields[i].fieldElem.Complexity()
	}
	return c
}

type structField struct {
	fieldTag  string // the string inside the `msgp:""` tag
	rawTag    string // the full tag (in case there are non-msgp keys)
	fieldName string // the name of the struct field
	fieldElem Elem   // the field type
}

// writeStructFields is a trampoline for writeBase for all of the fields in a struct.
func writeStructFields(s []structField, structName string) {
	for i := range s {
		s[i].fieldElem.SetVarname(fmt.Sprintf("%s.%s", structName, s[i].fieldName))
	}
}

// ShimMode determines whether the shim is a cast or a convert.
type ShimMode uint8

const (
	// Cast says to use the casting mode.
	Cast ShimMode = 0

	// Convert says to use the conversion mode.
	Convert ShimMode = 1
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

// Printable says if the element is printable.
func (s *BaseElem) Printable() bool { return !s.mustinline }

// Alias sets an alias.
func (s *BaseElem) Alias(typ string) {
	s.common.Alias(typ)
	if s.Value != IDENT {
		s.Convert = true
	}
	if strings.Contains(typ, ".") {
		s.mustinline = true
	}
}

// SetVarname sets the name of the variable.
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

// TypeName returns the canonical Go type name for the base element.
func (s *BaseElem) TypeName() string {
	if s.common.alias != "" {
		return s.common.alias
	}
	s.common.Alias(s.BaseType())
	return s.common.alias
}

// ToBase is used as tmp = {{ToBase}}({{Varname}}) if Convert==true.
func (s *BaseElem) ToBase() string {
	if s.ShimToBase != "" {
		return s.ShimToBase
	}
	return s.BaseType()
}

// FromBase is used as {{Varname}} = {{FromBase}}(tmp) if Convert==true.
func (s *BaseElem) FromBase() string {
	if s.ShimFromBase != "" {
		return s.ShimFromBase
	}
	return s.TypeName()
}

func (s *BaseElem) toBaseConvert() string {
	return s.ToBase() + "(" + s.Varname() + ")"
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

// BaseType gives the name of the base type.
func (s *BaseElem) BaseType() string {
	switch s.Value {
	case IDENT:
		return s.TypeName()

	// Exceptions to the naming/capitalization rule:
	case Intf:
		return "interface{}"
	case Bytes:
		return "[]byte"
	case Time:
		return "time.Time"
	case Ext:
		return "msgp.Extension"

	// Everything else is base.String() with
	// the first letter as lowercase.
	default:
		return strings.ToLower(s.BaseName())
	}
}

// Needsref indicates whether the base type is a pointer.
func (s *BaseElem) Needsref(b bool) {
	s.needsref = b
}

// Copy returns a deep copy of the object.
func (s *BaseElem) Copy() Elem {
	g := *s
	return &g
}

// Complexity returns a measure of the complexity of the element.
func (s *BaseElem) Complexity() int {
	if s.Convert && !s.mustinline {
		return 2
	}
	// We need to return 1 if !printable() to make sure that
	// stuff gets inlined appropriately.
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
