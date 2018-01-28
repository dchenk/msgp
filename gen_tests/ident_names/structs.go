package ident_names

//go:generate msgp

type SmallStruct struct {
	Foo string
	Bar string
	Qux string
}

type BigStruct struct {
	Foo   string
	Bar   string
	Baz   []string
	Qux   map[string]string
	Yep   map[string]map[string]string
	Quack struct {
		Quack struct {
			Quack struct {
				Quack string
			}
		}
	}
	Nup struct {
		Foo string
		Bar string
		Baz []string
		Qux map[string]string
		Yep map[string]map[string]string
	}
	Ding struct {
		Dong struct {
			Dung struct {
				Thing string
			}
		}
	}
}

// OtherStruct is here to check how the code following changed code above is affected.
type OtherStruct struct {
	Str string
	Num uint32
	Map map[string]string
}
