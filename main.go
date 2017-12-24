// msgp is a code generation tool for creating methods to serialize and de-serialize Go
// data structures to and from MessagePack.
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
//  -file = input file name (or directory; default is $GOFILE, which is set by the `go generate` command)
//  -io = satisfy the `msgp.Decoder` and `msgp.Encoder` interfaces (default is true)
//  -marshal = satisfy the `msgp.Marshaler` and `msgp.Unmarshaler` interfaces (default is true)
//  -tests = generate tests and benchmarks (default is true)
//
// For more information, please read README.md, and the wiki at github.com/dchenk/msgp
//
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/dchenk/msgp/gen"
	"github.com/dchenk/msgp/parse"
	"github.com/ttacon/chalk"
	"golang.org/x/tools/imports"
)

var (
	out        = flag.String("o", "", "output file")
	file       = flag.String("file", "", "input file")
	encode     = flag.Bool("io", true, "create Encode and Decode methods")
	marshal    = flag.Bool("marshal", true, "create Marshal and Unmarshal methods")
	tests      = flag.Bool("tests", true, "create tests and benchmarks")
	unexported = flag.Bool("unexported", false, "also process unexported types")
)

func main() {

	flag.Parse()

	// GOFILE is set by go generate
	if *file == "" {
		*file = os.Getenv("GOFILE")
		if *file == "" {
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

	if mode&^gen.Test == 0 {
		fmt.Println(chalk.Red.Color("No methods to generate; -io=false && -marshal=false"))
		os.Exit(1)
	}

	if err := Run(*file, mode, *unexported); err != nil {
		fmt.Println(chalk.Red.Color(err.Error()))
		os.Exit(1)
	}

}

// Run writes all methods using the associated file or path, e.g.
//
//	err := msgp.Run("path/to/myfile.go", gen.Size|gen.Marshal|gen.Unmarshal|gen.Test, false)
//
func Run(gofile string, mode gen.Method, unexported bool) error {
	if mode&^gen.Test == 0 {
		return nil
	}
	fmt.Println(chalk.Magenta.Color("======== MessagePack Code Generator ======="))
	fmt.Printf(chalk.Magenta.Color(">>> Input: \"%s\"\n"), gofile)
	fs, err := parse.File(gofile, unexported)
	if err != nil {
		return err
	}

	if len(fs.Identities) == 0 {
		fmt.Println(chalk.Magenta.Color("No types requiring code generation were found!"))
		return nil
	}

	return printFile(newFilename(gofile, fs.Package), fs, mode)
}

// newFilename picks a new file name based on input flags and input file names.
func newFilename(old string, pkg string) string {

	if *out != "" {
		if pre := strings.TrimPrefix(*out, old); len(pre) > 0 &&
			!strings.HasSuffix(*out, ".go") {
			return filepath.Join(old, *out)
		}
		return *out
	}

	if fi, err := os.Stat(old); err == nil && fi.IsDir() {
		old = filepath.Join(old, pkg)
	}

	// new file name is old file name + _gen.go
	return strings.TrimSuffix(old, ".go") + "_gen.go"

}

// printFile prints the methods for the provided list of elements to the given file name and
// canonical package path.
func printFile(fileName string, f *parse.FileSet, mode gen.Method) error {

	out, tests, err := generate(f, mode)
	if err != nil {
		return err
	}

	// Write the file we want in one goroutine and its associated test file in another.
	doneErr := make(chan error, 1)
	go func() {
		doneErr <- formatWrite(fileName, out.Bytes())
	}()

	if tests != nil {
		testFileName := strings.TrimSuffix(fileName, ".go") + "_test.go"
		err = formatWrite(testFileName, tests.Bytes())
		if err != nil {
			return err
		}
		wroteInfo(testFileName)
	}

	err = <-doneErr
	if err != nil {
		return err
	}
	// If we got this far, then the file was written with no errors.
	wroteInfo(fileName)

	return nil

}

// formatWrite runs the imports formatter on data (representing a Go source file) and
// writes the output to a file at fileName, creating a file if nothing exists there.
func formatWrite(fileName string, data []byte) error {
	out, err := imports.Process(fileName, data, nil)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fileName, out, 0600)
}

// generate is generates all the desired methods and test functions for the file f and writes
// each file to a buffer.
func generate(f *parse.FileSet, mode gen.Method) (*bytes.Buffer, *bytes.Buffer, error) {

	mainBuf := bytes.NewBuffer(make([]byte, 0, 4096))
	writePkgHeader(mainBuf, f.Package)

	myImports := []string{"github.com/dchenk/msgp/msgp"}
	for _, imp := range f.Imports {
		if imp.Name != nil {
			// If the import has an alias, include it (imp.Path.Value is a quoted string).
			myImports = append(myImports, imp.Name.Name+" "+imp.Path.Value)
		} else {
			myImports = append(myImports, imp.Path.Value)
		}
	}

	// De-duplicate the imports.
	for i := 0; i < len(myImports); i++ {
		for j := range myImports {
			if myImports[i] == myImports[j] && i != j {
				myImports = append(myImports[:j], myImports[j+1:]...)
				i--
				break
			}
		}
	}

	writeImportHeader(mainBuf, myImports)

	var testsBuf *bytes.Buffer
	if mode&gen.Test == gen.Test {
		testsBuf = bytes.NewBuffer(make([]byte, 0, 4096))
		writePkgHeader(testsBuf, f.Package)
		neededImports := []string{"github.com/dchenk/msgp/msgp", "testing"}
		if mode&(gen.Encode|gen.Decode) != 0 {
			neededImports = append(neededImports, "bytes")
		}
		writeImportHeader(testsBuf, neededImports)
	}

	return mainBuf, testsBuf, f.PrintTo(gen.NewGeneratorSet(mode, mainBuf, testsBuf))

}

func writePkgHeader(b *bytes.Buffer, name string) {
	b.WriteString("package " + name)
	b.WriteString("\n// THIS FILE WAS PRODUCED BY THE MSGP CODE GENERATION TOOL (github.com/dchenk/msgp).\n// DO NOT EDIT.\n\n")
}

func writeImportHeader(b *bytes.Buffer, imports []string) {
	b.WriteString("import (\n")
	for _, im := range imports {
		if im[len(im)-1] == '"' {
			// This is an aliased import, so don't quote it again.
			fmt.Fprintf(b, "\t%s\n", im)
		} else {
			fmt.Fprintf(b, "\t%q\n", im)
		}
	}
	b.WriteString(")\n\n")
}

func wroteInfo(fileName string) {
	fmt.Printf(chalk.Magenta.Color(">>> Wrote and formatted \"%s\"\n"), fileName)
}
