// Package gen is the tool used to generate Go code for the types in your program that you want
// to efficiently serialize to and from the MessagePack format. This package is designed to be
// imported by the CLI tool located at the root of the dchenk/msgp repository, which is used by
// the Go code-generating tool "go generate".

module "github.com/dchenk/msgp/gen"

require (
	"github.com/dchenk/msgp" v0.0.0-20180420210123-e1e324a7758f
	"github.com/philhofer/fwd" v1.0.0
	"github.com/ttacon/chalk" v0.0.0-20160626202418-22c06c80ed31
	"golang.org/x/tools" v0.0.0-20180416195352-94b14834a201
)
