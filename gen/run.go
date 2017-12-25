// This package is the tool msgp uses to generate Go code for the types in your program that you
// want to serialize to and from the MessagePack format. The package is designed to be usable by
// both the main.go file at the root of this repository (installed as a command line tool that is
// called by the `go generate` command) and by external programs that import the package.
//
// Documentation on how to use this tool either by the command line or from a Go program is at the
// wiki at https://github.com/dchenk/msgp/wiki.
//
// To use this package from a Go program, call Run on a file or directory with the settings you want.
// Example:
//
//  import "github.com/dchenk/msgp/gen"
//
//  err := gen.Run("path/to/myfile.go", gen.Size|gen.Marshal|gen.Unmarshal|gen.Test, false)
//
package gen

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ttacon/chalk"
	"golang.org/x/tools/imports"
)

// Run writes your desired methods and test files. You must set the source code path. The output file
// path can be left blank to have a file created at old_name_gen.go (_gen appended to the old name; the
// test file, if you opt to create one, will be at old_name_gen_test.go). The mode is the set of Method
// types and tests you would like. Set unexported to true if you want code to be generated for unexported
// as well as for exported types.
func Run(srcPath string, outputPath string, mode Method, unexported bool) error {

	if mode&^Test == 0 {
		return errors.New("no methods to generate; -io=false and -marshal=false")
	}

	s, err := newSource(srcPath, unexported)
	if err != nil {
		return err
	}

	if len(s.identities) == 0 {
		return errors.New("no types requiring code generation were found")
	}

	fmt.Println(chalk.Magenta.Color("======= MessagePack Code Generating ======="))
	fmt.Printf(chalk.Magenta.Color(">>> Input: \"%s\"\n"), srcPath)

	return printFile(newFilename(s.pkg, srcPath, outputPath), s, mode)

}

// printFile prints the methods for the provided list of methods at the given file name.
func printFile(fileName string, s *source, mode Method) error {

	mainBuf := bytes.NewBuffer(make([]byte, 0, 4096))
	writePkgHeader(mainBuf, s.pkg)

	mainImports := []string{"github.com/dchenk/msgp/msgp"}
	for _, imp := range s.imports {
		if imp.Name != nil {
			// If the import has an alias, include it (imp.Path.Value is a quoted string).
			mainImports = append(mainImports, imp.Name.Name+" "+imp.Path.Value)
		} else {
			mainImports = append(mainImports, imp.Path.Value)
		}
	}

	// De-duplicate the imports.
	for i := 0; i < len(mainImports); i++ {
		for j := range mainImports {
			if mainImports[i] == mainImports[j] && i != j {
				mainImports = append(mainImports[:j], mainImports[j+1:]...)
				i--
				break
			}
		}
	}

	writeImportHeader(mainBuf, mainImports)

	var testsBuf *bytes.Buffer
	if mode&Test == Test {
		testsBuf = bytes.NewBuffer(make([]byte, 0, 4096))
		writePkgHeader(testsBuf, s.pkg)
		neededImports := []string{"github.com/dchenk/msgp/msgp", "testing"}
		if mode&(Encode|Decode) != 0 {
			neededImports = append(neededImports, "bytes")
		}
		writeImportHeader(testsBuf, neededImports)
	}

	if err := s.printTo(newGeneratorSet(mode, mainBuf, testsBuf)); err != nil {
		return err
	}

	// Write the methods file concurrently with its associated test file.
	doneErr := make(chan error, 1)
	go func() {
		doneErr <- formatWrite(fileName, mainBuf.Bytes())
	}()

	if testsBuf != nil {
		testFileName := strings.TrimSuffix(fileName, ".go") + "_test.go"
		if err := formatWrite(testFileName, testsBuf.Bytes()); err != nil {
			return err
		}
	}

	return <-doneErr

}

// newFilename picks a new file name based on input flags and the old name.
func newFilename(pkg, old, new string) string {

	if new != "" {
		pre := strings.TrimPrefix(new, old)
		if len(pre) > 0 && !strings.HasSuffix(new, ".go") {
			return filepath.Join(old, new)
		}
		return new
	}

	if fi, err := os.Stat(old); err == nil && fi.IsDir() {
		old = filepath.Join(old, pkg)
	}

	// The new file name is old name + _gen.go
	return strings.TrimSuffix(old, ".go") + "_gen.go"

}

// formatWrite runs the imports formatter on data (representing a Go source file) and
// writes the output to a file at fileName, creating a file if nothing exists there.
func formatWrite(fileName string, data []byte) error {
	out, err := imports.Process(fileName, data, nil)
	if err != nil {
		return err
	}
	fmt.Printf(chalk.Magenta.Color(">>> Writing file \"%s\"\n"), fileName)
	return ioutil.WriteFile(fileName, out, 0600)
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
