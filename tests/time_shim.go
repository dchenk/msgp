package tests

import "time"

//go:generate msgp

// The following line will generate an error after the code is generated
// if the generated code doesn't have the identifier timetostr in it.

//go:generate ./time_shim_search.sh $GOFILE timetostr

//msgp:shim time.Time as:string using:timetostr/strtotime

// T represents a type that we'll be shimming.
type T struct {
	T time.Time
}

func timetostr(t time.Time) string {
	return t.Format(time.RFC3339)
}

func strtotime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
