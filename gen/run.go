// Package gen is the tool msgp uses to generate Go code for the types in your program that you
// want to serialize to and from the MessagePack format. This package is designed to be usable by
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
//  err := gen.Run("path/to/my_file.go", gen.Size|gen.Marshal|gen.Unmarshal|gen.Test, false)
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

	mainBuf, testsBuf, err := RunData(srcPath, mode, unexported)
	if err != nil {
		return err
	}

	if outputPath != "" {
		pre := strings.TrimPrefix(outputPath, srcPath)
		if len(pre) > 0 && !strings.HasSuffix(outputPath, ".go") {
			outputPath = filepath.Join(srcPath, outputPath)
		}
	} else if stat, err := os.Stat(srcPath); err == nil && stat.IsDir() {
		// The new file is named msgp_gen.go in the source directory.
		outputPath = filepath.Join(srcPath, "msgp_gen.go")
	} else {
		// The new file name is the source file name + _gen.go
		outputPath = strings.TrimSuffix(srcPath, ".go") + "_gen.go"
	}

	// Write the methods file concurrently with its associated test file.
	doneErr := make(chan error, 1)
	go func() {
		doneErr <- formatWrite(outputPath, mainBuf.Bytes())
	}()

	if testsBuf != nil {
		testFileName := strings.TrimSuffix(outputPath, ".go") + "_test.go"
		if err := formatWrite(testFileName, testsBuf.Bytes()); err != nil {
			return err
		}
	}

	return <-doneErr

}

// RunData works just like Run except that, instead of writing out a file, it outputs the generated file's contents,
// the corresponding generated test file (nil if mode does not include gen.Test), and a possibly nil error.
func RunData(srcPath string, mode Method, unexported bool) (mainBuf *bytes.Buffer, testsBuf *bytes.Buffer, err error) {

	if mode&^Test == 0 {
		err = errors.New("no methods to generate; -io=false and -marshal=false")
		return
	}

	s, err := newSource(srcPath, unexported)
	if err != nil {
		return
	}

	if len(s.identities) == 0 {
		err = errors.New("no types requiring code generation were found")
		return
	}

	fmt.Println(chalk.Magenta.Color("======= MessagePack Code Generating ======="))
	fmt.Printf(chalk.Magenta.Color("   Input: %s\n"), srcPath)

	mainBuf = bytes.NewBuffer(make([]byte, 0, 4096))
	writePkgHeader(mainBuf, s.pkg)

	mainImports := []string{"github.com/dchenk/msgp/msgp"}
	for _, imp := range s.imports {
		if imp.Name != nil {
			// If the import has an alias, include it (imp.Path.Value is a quoted string).
			// But do not include the import if its alias is the blank identifier.
			if imp.Name.Name == "_" {
				fmt.Printf(chalk.Blue.Color("Not including import %s with blank identifier as alias.\n"), imp.Path.Value)
			} else {
				mainImports = append(mainImports, imp.Name.Name+" "+imp.Path.Value)
			}
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

	// Write the test file if it's desired.
	if mode&Test == Test {
		testsBuf = bytes.NewBuffer(make([]byte, 0, 4096))
		writePkgHeader(testsBuf, s.pkg)
		neededImports := []string{"github.com/dchenk/msgp/msgp", "testing"}
		if mode&(Encode|Decode) != 0 {
			neededImports = append(neededImports, "bytes")
		}
		writeImportHeader(testsBuf, neededImports)
	}

	err = s.printTo(newGeneratorSet(mode, mainBuf, testsBuf))

	return

}

// formatWrite runs the imports formatter on data (representing a Go source file) and
// writes the output to a file at fileName, creating a file if nothing exists there.
func formatWrite(fileName string, data []byte) error {
	out, err := imports.Process(fileName, data, nil)
	if err != nil {
		return err
	}
	fmt.Printf(chalk.Magenta.Color("   Writing file: %s\n"), fileName)
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
