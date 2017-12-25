package gen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strings"
)

// A source is the in-memory representation of a either a single parsed source code
// file or a concatenation of source code files.
type source struct {
	pkg        string              // package name
	specs      map[string]ast.Expr // type specs found in the code
	identities map[string]Elem     // identities processed from specs
	directives []string            // raw preprocessor directives (lines of comments)
	imports    []*ast.ImportSpec   // imports
}

// newSource parses a file at the path provided and produces a new *source.
// If srcPath is the path to a directory, the entire directory will be parsed.
// If unexported is false, only exported identifiers are included in the source.
// If the resulting source would be empty, an error is returned.
func newSource(srcPath string, unexported bool) (*source, error) {

	pushState(srcPath)
	defer popState()
	s := &source{
		specs:      make(map[string]ast.Expr),
		identities: make(map[string]Elem),
	}

	stat, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	if stat.IsDir() {
		pkgs, err := parser.ParseDir(fset, srcPath, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		if len(pkgs) != 1 {
			return nil, fmt.Errorf("multiple packages in directory: %s", srcPath)
		}
		// Extract the Package from the pkgs map.
		var pkg *ast.Package
		for n := range pkgs {
			s.pkg = n
			pkg = pkgs[n]
			break
		}
		for _, fl := range pkg.Files {
			pushState(fl.Name.Name)
			s.directives = append(s.directives, yieldComments(fl.Comments)...)
			if !unexported {
				ast.FileExports(fl)
			}
			s.getTypeSpecs(fl)
			popState()
		}
	} else {
		f, err := parser.ParseFile(fset, srcPath, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		s.pkg = f.Name.Name
		s.directives = yieldComments(f.Comments)
		if !unexported {
			ast.FileExports(f)
		}
		s.getTypeSpecs(f)
	}

	if len(s.specs) == 0 {
		return nil, fmt.Errorf("no definitions in %s", srcPath)
	}

	s.process()
	s.applyDirectives()
	s.propInline()

	return s, nil

}

func (s *source) printTo(gs generatorSet) error {
	s.applyDirs(gs)
	names := make([]string, 0, len(s.identities))
	for name := range s.identities {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		el := s.identities[name]
		el.SetVarname("z")
		pushState(el.TypeName())
		err := gs.Print(el)
		popState()
		if err != nil {
			return err
		}
	}
	return nil
}

// applyDirectives applies all of the directives that are known to the parser.
// Additional method-specific directives remain in s.directives.
func (s *source) applyDirectives() {
	newdirs := make([]string, 0, len(s.directives))
	for _, d := range s.directives {
		chunks := strings.Split(d, " ")
		if len(chunks) > 0 {
			if fn, ok := directives[chunks[0]]; ok {
				pushState(chunks[0])
				if err := fn(chunks, s); err != nil {
					warnln(err.Error())
				}
				popState()
			} else {
				newdirs = append(newdirs, d)
			}
		}
	}
	s.directives = newdirs
}

// A linkset is a graph of unresolved identities.
//
// Since Ident can only represent one level of type
// indirection (e.g. Foo -> uint8), type declarations like `type Foo Bar`
// aren't resolve-able until we've processed everything else.
//
// The goal of this dependency resolution is to distill the type
// declaration into just one level of indirection.
// In other words, if we have:
//
//  type A uint64
//  type B A
//  type C B
//  type D C
//
// then we want to end up figuring out that D is just a uint64.
type linkset map[string]*BaseElem

func (s *source) resolve(ls linkset) {

	progress := true
	for progress && len(ls) > 0 {
		progress = false
		for name, elem := range ls {
			n, ok := s.identities[elem.TypeName()]
			if ok {
				// Copy the old type descriptor, alias it
				// to the new value, and insert it into
				// the resolved identities list.
				progress = true
				nt := n.Copy()
				nt.Alias(name)
				s.identities[name] = nt
				delete(ls, name)
			}
		}
	}

	// What's left can't be resolved.
	for name, elem := range ls {
		warnf("couldn't resolve type %s (%s)\n", name, elem.TypeName())
	}

}

// process takes the contents of f.Specs and
// uses them to populate f.Identities
func (s *source) process() {

	deferred := make(linkset)
parse:
	for name, def := range s.specs {
		pushState(name)
		el := s.parseExpr(def)
		if el == nil {
			warnln("failed to parse")
			popState()
			continue parse
		}
		// push unresolved identities into
		// the graph of links and resolve after
		// we've handled every possible named type.
		if be, ok := el.(*BaseElem); ok && be.Value == IDENT {
			deferred[name] = be
			popState()
			continue parse
		}
		el.Alias(name)
		s.identities[name] = el
		popState()
	}

	if len(deferred) > 0 {
		s.resolve(deferred)
	}
}

func strToMethod(s string) Method {
	switch s {
	case "encode":
		return Encode
	case "decode":
		return Decode
	case "test":
		return Test
	case "size":
		return Size
	case "marshal":
		return Marshal
	case "unmarshal":
		return Unmarshal
	default:
		return 0
	}
}

func (s *source) applyDirs(p generatorSet) {
	// apply directives of the form
	//
	// 	//msgp:encode ignore {{TypeName}}
	//
loop:
	for _, d := range s.directives {
		chunks := strings.Split(d, " ")
		if len(chunks) > 1 {
			for i := range chunks {
				// Remove spacing around each word (type name) in
				// case there is any spacing.
				chunks[i] = strings.TrimSpace(chunks[i])
			}
			m := strToMethod(chunks[0]) // m is the directive's Method
			if m == 0 {
				warnf("unknown pass name: %q\n", chunks[0])
				continue loop
			}
			if fn, ok := passDirectives[chunks[1]]; ok {
				pushState(chunks[1])
				err := fn(m, chunks[2:], p)
				if err != nil {
					warnf("error applying directive: %s\n", err)
				}
				popState()
			} else {
				warnf("unrecognized directive %q\n", chunks[1])
			}
		} else {
			warnf("empty directive: %q\n", d)
		}
	}
}

// getTypeSpecs extracts all of the *ast.TypeSpecs in the file
// into s.identities but does not set the actual element.
func (s *source) getTypeSpecs(f *ast.File) {

	// Collect all imports.
	s.imports = append(s.imports, f.Imports...)

	// Check all declarations.
	for i := range f.Decls {

		if g, ok := f.Decls[i].(*ast.GenDecl); ok {

			// Check the specs.
			for _, spec := range g.Specs {

				if ts, ok := spec.(*ast.TypeSpec); ok {
					switch ts.Type.(type) { // These are the parse-able type specs.
					case *ast.StructType,
						*ast.ArrayType,
						*ast.StarExpr,
						*ast.MapType,
						*ast.Ident:
						s.specs[ts.Name.Name] = ts.Type
					}
				}

			}
		}
	}
}

func fieldName(f *ast.Field) string {
	l := len(f.Names)
	if l == 0 {
		return stringify(f.Type)
	} else if l == 1 {
		return f.Names[0].Name
	}
	return f.Names[0].Name + " (and others)"
}

func (s *source) parseFieldList(fl *ast.FieldList) []StructField {
	if fl == nil || fl.NumFields() == 0 {
		return nil
	}
	out := make([]StructField, 0, fl.NumFields())
	for _, field := range fl.List {
		pushState(fieldName(field))
		fds := s.getField(field)
		if len(fds) > 0 {
			out = append(out, fds...)
		} else {
			warnln("ignored.")
		}
		popState()
	}
	return out
}

// translate *ast.Field into []StructField
func (s *source) getField(f *ast.Field) []StructField {

	fields := make([]StructField, 1)
	var extension bool
	// parse tag; otherwise field name is field tag
	if f.Tag != nil {
		body := reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("msg")
		tags := strings.Split(body, ",")
		if len(tags) == 2 && tags[1] == "extension" {
			extension = true
		}
		// ignore "-" fields
		if tags[0] == "-" {
			return nil
		}
		fields[0].FieldTag = tags[0]
		fields[0].RawTag = f.Tag.Value
	}

	ex := s.parseExpr(f.Type)
	if ex == nil {
		return nil
	}

	// parse field name
	switch len(f.Names) {
	case 0:
		fields[0].FieldName = embedded(f.Type)
	case 1:
		fields[0].FieldName = f.Names[0].Name
	default:
		// this is for a multiple in-line declaration,
		// e.g. type A struct { One, Two int }
		fields = fields[0:0]
		for _, nm := range f.Names {
			fields = append(fields, StructField{
				FieldTag:  nm.Name,
				FieldName: nm.Name,
				FieldElem: ex.Copy(),
			})
		}
		return fields
	}
	fields[0].FieldElem = ex
	if fields[0].FieldTag == "" {
		fields[0].FieldTag = fields[0].FieldName
	}

	// Validate the extension.
	if extension {
		switch ex := ex.(type) {
		case *Ptr:
			if b, ok := ex.Value.(*BaseElem); ok {
				b.Value = Ext
			} else {
				warnln("Couldn't cast to extension.")
				return nil
			}
		case *BaseElem:
			ex.Value = Ext
		default:
			warnln("Couldn't cast to extension.")
			return nil
		}
	}

	return fields

}

// Extract embedded field names.
// So for a struct like
//
//	type A struct {
//		io.Writer
//  }
//
// we want "Writer"
func embedded(f ast.Expr) string {
	switch f := f.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.StarExpr:
		return embedded(f.X)
	case *ast.SelectorExpr:
		return f.Sel.Name
	default:
		// Nothing else is allowed.
		return ""
	}
}

// stringify a field type name
func stringify(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + stringify(e.X)
	case *ast.SelectorExpr:
		return stringify(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + stringify(e.Elt)
		}
		return fmt.Sprintf("[%s]%s", stringify(e.Len), stringify(e.Elt))
	case *ast.InterfaceType:
		if e.Methods == nil || e.Methods.NumFields() == 0 {
			return "interface{}"
		}
	}
	return "<BAD>"
}

// recursively translate ast.Expr to Elem; nil means type not supported.
// Expected input types:
// - *ast.MapType (map[T]J)
// - *ast.Ident (name)
// - *ast.ArrayType ([(sz)]T)
// - *ast.StarExpr (*T)
// - *ast.StructType (struct {})
// - *ast.SelectorExpr (a.B)
// - *ast.InterfaceType (interface {})
func (s *source) parseExpr(e ast.Expr) Elem {
	switch e := e.(type) {

	case *ast.MapType:
		if k, ok := e.Key.(*ast.Ident); ok && k.Name == "string" {
			if in := s.parseExpr(e.Value); in != nil {
				return &Map{Value: in}
			}
		}
		return nil

	case *ast.Ident:
		b := Ident(e.Name)

		// Work to resolve this expression can be done later,
		// once we've resolved everything else.
		if b.Value == IDENT {
			if _, ok := s.specs[e.Name]; !ok {
				warnf("non-local identifier: %s\n", e.Name)
			}
		}
		return b

	case *ast.ArrayType:

		// special case for []byte
		if e.Len == nil {
			if i, ok := e.Elt.(*ast.Ident); ok && i.Name == "byte" {
				return &BaseElem{Value: Bytes}
			}
		}

		// Return early if we don't know the slice element type.
		els := s.parseExpr(e.Elt)
		if els == nil {
			return nil
		}

		// array and not a slice
		if e.Len != nil {
			switch lt := e.Len.(type) {
			case *ast.BasicLit:
				return &Array{
					Size: lt.Value,
					Els:  els,
				}

			case *ast.Ident:
				return &Array{
					Size: lt.String(),
					Els:  els,
				}

			case *ast.SelectorExpr:
				return &Array{
					Size: stringify(lt),
					Els:  els,
				}

			default:
				return nil
			}
		}
		return &Slice{Els: els}

	case *ast.StarExpr:
		if v := s.parseExpr(e.X); v != nil {
			return &Ptr{Value: v}
		}
		return nil

	case *ast.StructType:
		return &Struct{Fields: s.parseFieldList(e.Fields)}

	case *ast.SelectorExpr:
		return Ident(stringify(e))

	case *ast.InterfaceType:
		// support `interface{}`
		if len(e.Methods.List) == 0 {
			return &BaseElem{Value: Intf}
		}
		return nil

	default: // other types not supported
		return nil
	}
}
