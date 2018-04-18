package gen

import (
	"fmt"
	"go/ast"
	"strings"
)

const linePrefix = "//msgp:"

// A directive takes arguments (possibly including an pass-name directive) and
// a *source to work with.
type directive func([]string, *source) error

// func(passName, args, generatorSet)
type passDirective func(Method, []string, generatorSet) error

// directives lists all recognized directives.
// To add a directive, define a `directive` func and add it to this list.
var directives = map[string]directive{
	"shim":   applyShim,
	"ignore": ignore,
	"tuple":  astuple,
}

// passDirectives lists the directives that can be used with a named pass.
// See func applyDirs for more info.
var passDirectives = map[string]passDirective{
	"ignore": passIgnore,
}

func passIgnore(m Method, typeNamePatterns []string, gs generatorSet) error {
	pushState(m.String())
	for _, tn := range typeNamePatterns {
		gs.ApplyDirective(m, IgnoreTypename(tn))
		infof("ignoring %s\n", tn)
	}
	popState()
	return nil
}

// yieldComments finds all comment lines that begin with //msgp:
func yieldComments(c []*ast.CommentGroup) (comments []string) {
	for _, cg := range c {
		for _, line := range cg.List {
			if strings.HasPrefix(line.Text, linePrefix) {
				comments = append(comments, strings.TrimPrefix(line.Text, linePrefix))
			}
		}
	}
	return
}

// applyShim applies a shim of the form:
// msgp:shim {Type} as:{Newtype} using:{toFunc/fromFunc} mode:{Mode}
func applyShim(text []string, s *source) error {
	if len(text) < 4 || len(text) > 5 {
		return fmt.Errorf("shim directive should have 3 or 4 arguments; found %d", len(text)-1)
	}

	name := text[1]
	be := Ident(strings.TrimPrefix(strings.TrimSpace(text[2]), "as:")) // parse as::{base}
	if name[0] == '*' {
		name = name[1:]
		be.Needsref(true)
	}
	be.Alias(name)

	usestr := strings.TrimPrefix(strings.TrimSpace(text[3]), "using:") // parse using::{method/method}

	methods := strings.Split(usestr, "/")
	if len(methods) != 2 {
		return fmt.Errorf("expected 2 using::{} methods; found %d (%q)", len(methods), text[3])
	}

	be.ShimToBase = methods[0]
	be.ShimFromBase = methods[1]

	if len(text) == 5 {
		modestr := strings.TrimPrefix(strings.TrimSpace(text[4]), "mode:") // parse mode::{mode}
		switch modestr {
		case "cast":
			be.ShimMode = Cast
		case "convert":
			be.ShimMode = Convert
		default:
			return fmt.Errorf("invalid shim mode; found %s, expected 'cast' or 'convert", modestr)
		}
	}

	infof("%s -> %s\n", name, be.Value.String())
	s.findShim(name, be)

	return nil
}

//msgp:ignore {TypeA} {TypeB}...
// Parameter "text" includes the string "ignore" and possibly more
// strings that represent either exact type names or regexp patterns.
func ignore(text []string, s *source) error {
	if len(text) < 2 {
		return nil
	}
	for _, typeNamePattern := range text[1:] {
		typeNamePattern = strings.TrimSpace(typeNamePattern)
		for k := range s.identities {
			if typeNameMatches(typeNamePattern, s.identities[k].TypeName()) {
				infof("ignoring %s\n", s.identities[k].TypeName())
				delete(s.identities, k)
			}
		}
	}
	return nil
}

//msgp:tuple {TypeA} {TypeB}...
func astuple(text []string, s *source) error {
	if len(text) < 2 {
		return nil
	}
	for _, item := range text[1:] {
		name := strings.TrimSpace(item)
		if el, ok := s.identities[name]; ok {
			if st, ok := el.(*Struct); ok {
				st.AsTuple = true
				infoln(name)
			} else {
				warnf("%s: only structs can be tuples\n", name)
			}
		}
	}
	return nil
}
