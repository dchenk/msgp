package msgp_test

import (
	"fmt"
	"os"

	"github.com/dchenk/msgp/msgp"
)

func ExampleEncode() {
	// Write out a message to a file:
	file, err := os.Create("my-message")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	var theMessage msgp.Number
	theMessage.AsInt(457)
	err = msgp.Encode(file, &theMessage)
	if err != nil {
		panic(err)
	}
}

func ExampleDecode() {
	// Read a message from a file:
	file, err := os.Open("my-message")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	var theMessage msgp.Number
	err = msgp.Decode(file, &theMessage)
	if err != nil {
		panic(err)
	}
	fmt.Print(theMessage.Int())
}
