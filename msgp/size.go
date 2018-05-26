package msgp

// The sizes here are the worst-case (biggest) encoded sizes for each type, including the
// prefix with the type information. For variable-length types like slices and strings,
// the total encoded size is the prefix size plus the length of the actual object.
const (
	Int8Size       = 2
	Int16Size      = 3
	Int32Size      = 5
	Int64Size      = 9
	Uint8Size      = 2
	Uint16Size     = 3
	Uint32Size     = 5
	Uint64Size     = 9
	IntSize        = Int64Size
	UintSize       = Uint64Size
	Float64Size    = 9
	Float32Size    = 5
	Complex64Size  = 10
	Complex128Size = 18

	ByteSize = 2
	BoolSize = 1
	NilSize  = 1
	TimeSize = 15

	MapHeaderSize   = 5
	ArrayHeaderSize = 5

	BytesPrefixSize     = 5
	StringPrefixSize    = 5
	ExtensionPrefixSize = 6
)
