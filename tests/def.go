package tests

import (
	"os"
	"time"

	"github.com/dchenk/msgp/msgp"
)

//go:generate msgp

// All of the struct definitions in this file are fed to the code generator
// when `make test` is called. A simple way to test a struct definition is
// to add it to this file.

type Block [32]byte

// X tests edge-cases with compiling size compilation.
type X struct {
	Values    [32]byte    // should compile to 32*msgp.ByteSize; encoded as Bin
	ValuesPtr *[32]byte   // check (*)[:] deref
	More      Block       // should be identical to the above
	Others    [][32]int32 // should compile to len(x.Others)*32*msgp.Int32Size
	Matrix    [][]int32   // should not optimize
	ManyFixed []Fixed
}

// Fixed tests fixed-size structs.
type Fixed struct {
	A float64
	B bool
}

// TestType tests various kinds of types.
type TestType struct {
	F   *float64          `msgp:"float"`
	Els map[string]string `msgp:"elements"`
	Obj struct {          // a nested anonymous struct
		ValueA string `msgp:"value_a"`
		ValueB []byte `msgp:"value_b"`
	} `msgp:"object"`
	Child      *TestType   `msgp:"child"`
	Time       time.Time   `msgp:"time"`
	Any        interface{} `msgp:"any"`
	Appended   msgp.Raw    `msgp:"appended"`
	Num        msgp.Number `msgp:"num"`
	Byte       byte
	Rune       rune
	RunePtr    *rune
	RunePtrPtr **rune
	RuneSlice  []rune
	Slice1     []string
	Slice2     []string
	SlicePtr   *[]string
}

//msgp:tuple Object
type Object struct {
	ObjectNo string   `msgp:"objno"`
	Slice1   []string `msgp:"slice1"`
	Slice2   []string `msgp:"slice2"`
	MapMap   map[string]map[string]string
}

//msgp:tuple TestBench
type TestBench struct {
	Name     string
	BirthDay time.Time
	Phone    string
	Siblings int
	Spouse   bool
	Money    float64
}

//msgp:tuple TestFast
type TestFast struct {
	Lat, Long, Alt float64 // inline declaration
	Data           []byte
}

// FastAlias tests nested aliases.
type FastAlias TestFast

// AliasContainer wraps a FastAlias.
type AliasContainer struct {
	Fast FastAlias
}

// Test dependency resolution
type IntA int
type IntB IntA
type IntC IntB

// TestHidden has an ignored field.
type TestHidden struct {
	A   string
	B   []float64
	Bad func(string) bool // This results in a warning: field "Bad" unsupported
}

// Embedded has a recursive pointer.
type Embedded struct {
	*Embedded   // Test an embedded field.
	Children    []Embedded
	PtrChildren []*Embedded
	Other       string
}

// Things has a few interesting things.
type Things struct {
	Cmplx complex64                         `msgp:"complex"` // test slices
	Vals  []int32                           `msgp:"values"`
	Arr   [msgp.ExtensionPrefixSize]float64 `msgp:"arr"`            // test const array and *ast.SelectorExpr as array size
	Arr2  [4]float64                        `msgp:"arr2"`           // test basic lit array
	Ext   *msgp.RawExtension                `msgp:"ext,extension"`  // test extension
	Oext  msgp.RawExtension                 `msgp:"oext,extension"` // test extension reference
}

// NoFields gets methods generated, but it does not have any fields that
// can get encoded or decoded with MessagePack.
//
// This tests whether a "field" variable declaration is made in unmarshal or
// decode methods for a struct with no encodable/decodable fields. If an unused
// "field" variable is declared, the code will not compile.
type NoFields struct {
	Func  func(string) error
	stuff int
}

// Test the shim directive:

//msgp:shim SpecialID as:[]byte using:toBytes/fromBytes

type SpecialID string
type TestObj struct{ ID1, ID2 SpecialID }

func toBytes(id SpecialID) []byte   { return []byte(string(id)) }
func fromBytes(id []byte) SpecialID { return SpecialID(string(id)) }

type MyEnum byte

const (
	A MyEnum = iota
	B
	C
	D
	invalid
)

//msgp:shim MyEnum as:string using:(MyEnum).String/myenumStr

//msgp:shim *os.File as:string using:filetostr/filefromstr

func filetostr(f *os.File) string {
	return f.Name()
}

func filefromstr(s string) *os.File {
	f, _ := os.Open(s)
	return f
}

func (m MyEnum) String() string {
	switch m {
	case A:
		return "A"
	case B:
		return "B"
	case C:
		return "C"
	case D:
		return "D"
	default:
		return "<invalid>"
	}
}

func myenumStr(s string) MyEnum {
	switch s {
	case "A":
		return A
	case "B":
		return B
	case "C":
		return C
	case "D":
		return D
	default:
		return invalid
	}
}

// Test pass-specific directives:

//msgp:decode ignore Insane

type Insane [3]map[string]struct{ A, B CustomInt }

type Custom struct {
	Bts   CustomBytes          `msgp:"bts"`
	Mp    map[string]*Embedded `msgp:"mp"`
	Enums []MyEnum             `msgp:"enums"` // test explicit enum shim
	Some  FileHandle           `msgp:"file_handle"`
}

type Files []*os.File

type FileHandle struct {
	Relevant Files  `msgp:"files"`
	Name     string `msgp:"name"`
}

type CustomInt int
type CustomBytes []byte

type Wrapper struct {
	Tree *Tree
}

type Tree struct {
	Children []Tree
	Element  int
	Parent   *Wrapper
}

// Ensure all different widths of integer can be used as constant keys.
const (
	ConstantInt    int    = 8
	ConstantInt8   int8   = 8
	ConstantInt16  int16  = 8
	ConstantInt32  int32  = 8
	ConstantInt64  int64  = 8
	ConstantUint   uint   = 8
	ConstantUint8  uint8  = 8
	ConstantUint16 uint16 = 8
	ConstantUint32 uint32 = 8
	ConstantUint64 uint64 = 8
)

type ArrayConstants struct {
	ConstantInt    [ConstantInt]string
	ConstantInt8   [ConstantInt8]string
	ConstantInt16  [ConstantInt16]string
	ConstantInt32  [ConstantInt32]string
	ConstantInt64  [ConstantInt64]string
	ConstantUint   [ConstantUint]string
	ConstantUint8  [ConstantUint8]string
	ConstantUint16 [ConstantUint16]string
	ConstantUint32 [ConstantUint32]string
	ConstantUint64 [ConstantUint64]string
	ConstantHex    [0x16]string
	ConstantOctal  [07]string
}

// NonMsgpTags tests that non-msgp struct field tags work.
type NonMsgpTags struct {
	A      []string `json:"fooJSON" msg:"fooMsgp"`
	B      string   `json:"barJSON"`
	C      []string `json:"bazJSON" msg:"-"`
	Nested []struct {
		A          []string `json:"a"`
		B          string   `json:"b"`
		C          []string `json:"c"`
		VeryNested []struct {
			A []string `json:"a"`
			B []string `msgp:"bbbb" xml:"-"`
		}
	}
}
