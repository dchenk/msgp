package tests

//go:generate msgp

type EmptyStruct struct{}

type EmptyStructNested struct {
	A int
	X struct{}
	Y struct{}
	Z int
}

//msgp:tuple EmptyStructTuple

type EmptyStructTuple struct{}

//msgp:tuple EmptyStructTupleNested

type EmptyStructTupleNested struct {
	A int
	X struct{}
	Y struct{}
	Z int
}

type EmptyStructUses struct {
	Nested    EmptyStruct
	NestedPtr *EmptyStruct
}

//msgp:tuple EmptyStructTupleUsesTuple

type EmptyStructTupleUsesTuple struct {
	Nested    EmptyStructTuple
	NestedPtr *EmptyStructTuple
}

//msgp:tuple EmptyStructTupleUsesMap

type EmptyStructTupleUsesMap struct {
	Nested    EmptyStruct
	NestedPtr *EmptyStruct
}

type EmptyStructMapUsesTuple struct {
	Nested    EmptyStructTuple
	NestedPtr *EmptyStructTuple
}
