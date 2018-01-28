// msgp is a code generation tool for creating methods to serialize and de-serialize Go
// data values to and from MessagePack.
//
// This package is targeted at the `go generate` tool. To use it, include the following directive
// in a Go source file with types requiring source generation:
//
//     //go:generate msgp
//
// The go generate tool should set the proper environment variables for the generator to execute
// without any command-line flags. However, the following options are supported, if you need them:
//
//  -o = output file name (default is {input}_gen.go)
//  -src = input file name or directory (default is $GOFILE set by the `go generate` command)
//  -io = satisfy the `msgp.Decoder` and `msgp.Encoder` interfaces (default is true)
//  -marshal = satisfy the `msgp.Marshaler` and `msgp.Unmarshaler` interfaces (default is true)
//  -tests = generate tests and benchmarks (default is true)
//
// You can also import github.com/dchenk/msgp/gen and use the code generator from any of your Go programs.
//
// For more information, please read README.md and the wiki at github.com/dchenk/msgp
//
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dchenk/msgp/gen"
	"github.com/ttacon/chalk"
)

var (
	src        = flag.String("src", "", "input file or directory")
	out        = flag.String("o", "", "output file")
	encode     = flag.Bool("io", true, "create Encode and Decode methods")
	marshal    = flag.Bool("marshal", true, "create Marshal and Unmarshal methods")
	tests      = flag.Bool("tests", true, "create tests and benchmarks")
	unexported = flag.Bool("unexported", false, "also process unexported types")
)

func main() {

	flag.Parse()

	if *src == "" {
		// GOFILE is set by the go generate tool.
		*src = os.Getenv("GOFILE")
		if *src == "" {
			fmt.Println(chalk.Red.Color("No file to parse."))
			os.Exit(1)
		}
	}

	var mode gen.Method
	if *encode {
		mode |= (gen.Encode | gen.Decode | gen.Size)
	}
	if *marshal {
		mode |= (gen.Marshal | gen.Unmarshal | gen.Size)
	}
	if *tests {
		mode |= gen.Test
	}

	if err := gen.Run(*src, *out, mode, *unexported); err != nil {
		fmt.Println(chalk.Red.Color(err.Error()))
		os.Exit(1)
	}

}
