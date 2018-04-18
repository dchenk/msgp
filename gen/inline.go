package gen

// This file defines when and how we propagate type information
// from one type declaration to another. After the processing pass,
// every non-primitive type is marshalled/unmarshalled/etc through
// a function call. Here we propagate the type information into the
// caller's type tree if the child type is simple enough.
//
// For example, types like
//
//    type A [4]int
//
// will get pushed into parent methods,
// whereas types like
//
//    type B [3]map[string]struct{A, B [4]string}
//
// will not.

// maxComplex is an approximate measure
// of the number of children in a node.
const maxComplex = 5

// findShim begins recursive search for identities with the
// given name and replaces them with be.
func (s *source) findShim(id string, be *BaseElem) {
	for name, el := range s.identities {
		pushState(name)
		switch el := el.(type) {
		case *Struct:
			for i := range el.Fields {
				s.nextShim(&el.Fields[i].fieldElem, id, be)
			}
		case *Array:
			s.nextShim(&el.Els, id, be)
		case *Slice:
			s.nextShim(&el.Els, id, be)
		case *Map:
			s.nextShim(&el.Value, id, be)
		case *Ptr:
			s.nextShim(&el.Value, id, be)
		}
		popState()
	}
	// We'll need this at the top level as well.
	s.identities[id] = be
}

func (s *source) nextShim(ref *Elem, id string, be *BaseElem) {
	if (*ref).TypeName() == id {
		vn := (*ref).Varname()
		*ref = be.Copy()
		(*ref).SetVarname(vn)
	} else {
		switch el := (*ref).(type) {
		case *Struct:
			for i := range el.Fields {
				s.nextShim(&el.Fields[i].fieldElem, id, be)
			}
		case *Array:
			s.nextShim(&el.Els, id, be)
		case *Slice:
			s.nextShim(&el.Els, id, be)
		case *Map:
			s.nextShim(&el.Value, id, be)
		case *Ptr:
			s.nextShim(&el.Value, id, be)
		}
	}
}

// propInline identifies and in-lines candidates.
func (s *source) propInline() {
	for name, el := range s.identities {
		pushState(name)
		switch el := el.(type) {
		case *Struct:
			for i := range el.Fields {
				s.nextInline(&el.Fields[i].fieldElem, name)
			}
		case *Array:
			s.nextInline(&el.Els, name)
		case *Slice:
			s.nextInline(&el.Els, name)
		case *Map:
			s.nextInline(&el.Value, name)
		case *Ptr:
			s.nextInline(&el.Value, name)
		}
		popState()
	}
}

func (s *source) nextInline(ref *Elem, root string) {
	switch el := (*ref).(type) {
	case *BaseElem:
		// Ensure that we're not inlining a type into itself.
		typ := el.TypeName()
		if el.Value == IDENT && typ != root {
			if node, ok := s.identities[typ]; ok && node.Complexity() < maxComplex {

				infof("inlining %s\n", typ)

				// This should never happen; it will cause infinite recursion.
				if node == *ref {
					panic("Detected infinite recursion in inlining loop! Please file a bug at github.com/dchenk/msgp/issues")
				}

				*ref = node.Copy()
				s.nextInline(ref, node.TypeName())

			} else if !ok && !el.Resolved() {
				// At this point we are sure that we've got a type that is neither
				// a primitive, a library builtin, nor a processed type.
				warnf("Unresolved identifier: %s\n", typ)
			}
		}
	case *Struct:
		for i := range el.Fields {
			s.nextInline(&el.Fields[i].fieldElem, root)
		}
	case *Array:
		s.nextInline(&el.Els, root)
	case *Slice:
		s.nextInline(&el.Els, root)
	case *Map:
		s.nextInline(&el.Value, root)
	case *Ptr:
		s.nextInline(&el.Value, root)
	default:
		panic("bad elem type")
	}
}
