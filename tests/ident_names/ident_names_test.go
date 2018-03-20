package ident_names

// This test is supposed to ensure that identifiers (variable names) are generated on a per-method basis,
// so that if a struct type definition is changed somewhere (a field is added, removed, or changed), no
// other types need new code generated.
// Also ensure that no duplicate identifiers appear in a method (make shadowing impossible).
// Structs are currently processed alphabetically by msgp; this test relies on that feature.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"testing"

	"github.com/dchenk/msgp/gen"
)

func TestIdentNames(t *testing.T) {

	testCases := []struct {
		newSrc           string
		expectedToChange []string
	}{ // The "changed" files don't have a ".go" extension to avoid compiling (and compiling errors for repeated declarations).
		{"./structs_small_changed.gosrc", []string{"SmallStruct"}},
		{"./structs_big_changed.gosrc", []string{"BigStruct"}},
		{"./structs_both_changed.gosrc", []string{"SmallStruct", "BigStruct"}},
	}

	mode := gen.Decode | gen.Encode | gen.Size | gen.Marshal | gen.Unmarshal
	methods := []string{"DecodeMsg", "EncodeMsg", "Msgsize", "MarshalMsg", "UnmarshalMsg"}

	// Generate the code for the "original" file.
	buf1, _, err := gen.RunData("./structs.go", mode, false)
	if err != nil {
		t.Fatalf("error running gen; %s", err)
	}

	for indx, testCase := range testCases {

		// Extract the generated variable names, each mapped to its function name.
		vars1, err := extractVars(buf1.Bytes())
		if err != nil {
			t.Fatalf("(indx %d): could not extract vars2: %v", indx, err)
		}

		// Generate the code for the "changed" file.
		buf2, _, err := gen.RunData(testCase.newSrc, mode, false)
		if err != nil {
			t.Fatalf("error running gen; %s", err)
		}

		// Extract the generated variable names, each mapped to its function name.
		vars2, err := extractVars(buf2.Bytes())
		if err != nil {
			t.Fatalf("(indx %d): could not extract vars2: %v", indx, err)
		}

		// Ensure that the declared variable names inside each of the methods we expect to
		// change have actually changed.
		for _, structName := range testCase.expectedToChange {
			for _, methodType := range methods {

				methodName := structName + "." + methodType

				val1, val2 := vars1.Value(methodName), vars2.Value(methodName)

				// Some generated methods will not have variables declared inside (such as EncodeMsg), so check
				// if there even are any local variables in either method; if there are, the methods must be different.
				if (len(val1) > 0 || len(val2) > 0) && reflect.DeepEqual(val1, val2) {
					t.Log("val1:", val1)
					t.Log("val2:", val2)
					t.Fatalf("(indx %d): vars identical but expected vars to change for %s", indx, methodName)
				}

				delete(vars1, methodName)
				delete(vars2, methodName)

			}
		}

		// None of the remaining keys should have changed.
		for methodName := range vars1 {
			if !reflect.DeepEqual(vars1[methodName], vars2.Value(methodName)) {
				t.Fatalf("%d: vars changed but expected identical vars for %s", indx, methodName)
			}
			delete(vars1, methodName)
			delete(vars2, methodName)
		}

		if len(vars1) > 0 || len(vars2) > 0 {
			t.Fatalf("(indx %d): unexpected methods remaining", indx)
		}

	}

}

func TestIdentNamesShadowing(t *testing.T) {

	srcs := []string{"./structs.go", "./structs_small_changed.gosrc", "./structs_big_changed.gosrc", "./structs_both_changed.gosrc"}
	mode := gen.Decode | gen.Encode | gen.Size | gen.Marshal | gen.Unmarshal

	for indx, src := range srcs {

		// Generate the code for the file.
		buf, _, err := gen.RunData(src, mode, false)
		if err != nil {
			t.Fatalf("error running gen; %s", err)
		}

		// Extract the generated variable names, each mapped to its function name.
		vars, err := extractVars(buf.Bytes())
		if err != nil {
			t.Fatalf("(indx %d): could not extract vars: %v", indx, err)
		}

		count := 0
		for methodName, methodVars := range vars {

			sort.Strings(methodVars)

			// Sanity check to make sure the test expectations aren't broken.
			// If the prefix ever changes, this needs to change.
			for _, v := range methodVars {
				if v[0] == 'z' {
					count++
				}
			}

			for i := range methodVars {
				for j := range methodVars {
					if methodVars[i] == methodVars[j] && i != j {
						t.Fatalf("(indx %d): duplicate var %s in function %s", indx, methodVars[i], methodName)
					}
				}
			}

		}

		// One more sanity check: If no vars that start with 'z', this test is not working right.
		if count == 0 {
			t.Fatalf("(indx %d): no generated identifiers found", indx)
		}

	}

}

type extractedVars map[string][]string

func (e extractedVars) Value(key string) []string {
	if v, ok := e[key]; ok {
		return v
	}
	panic(fmt.Errorf("unknown key %s", key)) // Requested values should all exist.
}

// extractVars extracts all top-level declaration in fileData (representing Go source code) and adds
// each to an extractedVars map, with the keys being the qualified struct method names.
func extractVars(fileData []byte) (extractedVars, error) {

	fs := token.NewFileSet()

	f, err := parser.ParseFile(fs, "", fileData, 0)
	if err != nil {
		return nil, err
	}

	vars := make(extractedVars)
	for _, dec := range f.Decls {
		switch dt := dec.(type) {
		// The only expected top-level declarations are function declarations (methods on the exported types).
		case *ast.FuncDecl:
			structName := ""
			switch rt := dt.Recv.List[0].Type.(type) {
			case *ast.Ident:
				structName = rt.Name
			case *ast.StarExpr:
				structName = rt.X.(*ast.Ident).Name
			default:
				panic("unknown receiver type")
			}
			key := structName + "." + dt.Name.Name
			vis := &varVisitor{fset: fs}
			ast.Walk(vis, dt.Body)
			vars[key] = vis.vars
		}
	}

	return vars, nil

}

type varVisitor struct {
	vars []string
	fset *token.FileSet
}

// Visit implements the ast.Visitor interface. It is used to walk through all the nodes in an AST element
// and identify the generic declaration nodes to extract and add to the vars slice.
func (v *varVisitor) Visit(node ast.Node) (w ast.Visitor) {
	gd, ok := node.(*ast.GenDecl)
	if !ok {
		return v
	}
	for _, spec := range gd.Specs {
		if vSpec, ok := spec.(*ast.ValueSpec); ok { // Exclude import and type declarations.
			for _, n := range vSpec.Names {
				v.vars = append(v.vars, n.Name)
			}
		}
	}
	return v
}
