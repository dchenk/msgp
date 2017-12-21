// +build !appengine

package msgp

import "unsafe"

// The spec says int and uint are always the same size, but that int/uint size may not be machine word size.
const smallint = unsafe.Sizeof(int(0)) == 4
