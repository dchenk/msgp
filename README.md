# MessagePack Code Generator and Library for Go
[![Build Status](https://travis-ci.org/dchenk/msgp.svg?branch=master)](https://travis-ci.org/dchenk/msgp) 
[![Go Report Card](https://goreportcard.com/badge/github.com/dchenk/msgp)](https://goreportcard.com/badge/github.com/dchenk/msgp)
[![GoDoc](https://godoc.org/github.com/dchenk/msgp?status.svg)](https://godoc.org/github.com/dchenk/msgp/msgp)


This is a code generation tool and serialization library for MessagePack. You can read more about MessagePack [in the wiki](https://github.com/dchenk/msgp/wiki)
or at [msgpack.org](https://msgpack.org).

**This library is no longer being maintained. You should be using [Protocol Buffers](https://developers.google.com/protocol-buffers/) for binary data serialization**

### Top Features

- Use Go as your schema language
- Performance is amazing
- JSON [interoperability](https://godoc.org/github.com/dchenk/msgp/msgp#CopyToJSON)
- Type safety
- Support for complex type declarations
- Define your own [MessagePack extensions](https://github.com/dchenk/msgp/wiki/Using-Extensions)
- Automatic unit test and benchmark generation
- Native support for Go’s `time.Time`, `complex64`, and `complex128` types
- [Preprocessor directives](https://github.com/dchenk/msgp/wiki/Using-the-Code-Generator)
- Generation of both `[]byte`-oriented and `io.Reader/io.Writer`-oriented methods

### Quickstart

In a Go code source file, include the following directive:

```go
//go:generate msgp
```

Within the directory with the file where you placed that directive, run `go generate`.

The `msgp` command will tell the `go generate` tool to generate serialization and deserialization methods (functions) for all exported types in the file. These custom-built functions enable you to use package [`msgp`](https://godoc.org/github.com/dchenk/msgp/msgp) to efficiently encode objects to and from the MessagePack format.

You can [read more about the code generation options here](https://github.com/dchenk/msgp/wiki/Using-the-Code-Generator).

### Use

Struct field names can be set the same way as with the `encoding/json` package. For example:

```go
type Person struct {
    Name       string `msgp:"name"`
    Address    string `msgp:"address"`
    Age        int    `msgp:"age"`
    Hidden     string `msgp:"-"` // this field is ignored
    unexported bool              // this field is also ignored
}
```

(The struct field tags are optional.)

By default, the code generator will satisfy `msgp.Sizer`, `msgp.Encoder`, `msgp.Decoder`, `msgp.Marshaler`, and `msgp.Unmarshaler`.
You’ll often find that much marshalling and unmarshalling will be done with zero heap allocations.

Although `msgp.Marshaler` and `msgp.Unmarshaler` are similar to the standard library’s `json.Marshaler` and `json.Unmarshaler`,
`msgp.Encoder` and `msgp.Decoder` are useful for stream serialization. (`*msgp.Writer` and `*msgp.Reader` are essentially
protocol-aware versions of `*bufio.Writer` and `*bufio.Reader`.)

Consider the following:
```go
const Eight = 8
type MyInt int
type Data []byte

type Struct struct {
    Which  map[string]*MyInt `msgp:"which"`
    Other  Data              `msgp:"other"`
    Nums   [Eight]float64    `msgp:"nums"`
}
```
As long as the declarations of `MyInt` and `Data` are in the same file as `Struct`, the parser will determine that the type information
for `MyInt` and `Data` can be passed into the definition of `Struct` before its methods are generated.

#### Extensions

MessagePack supports defining your own types through "extensions," which are just a tuple of the data "type" (`int8`) and the raw binary.
You can see [a worked example in the wiki.](https://github.com/dchenk/msgp/wiki/Using-Extensions)

### Status

The code generator here and runtime library are both stable. Newer versions of the code may generate different code than older versions
for performance reasons.

You can read more about how `msgp` maps MessagePack types onto Go types [in the wiki](http://github.com/dchenk/msgp/wiki).

Here some of the known limitations/restrictions:

- Identifiers from outside the processed source file are assumed to satisfy the generator's interfaces. If this isn't the case, your code
will fail to compile.
- The `chan` and `func` fields and types are ignored as well as un-exported fields.
- Encoding of `interface{}` is limited to built-ins or types that have explicit encoding methods.
- Maps must have `string` keys. This is intentional (as it preserves JSON interoperability). Although non-string map keys are not forbidden
by the MessagePack standard, many serializers impose this restriction. (It also means *any* well-formed `struct` can be decoded into a
`map[string]interface{}`.) The only exception to this rule is that the decoders will allow you to read map keys encoded as `bin` types,
since some legacy encodings permitted this. (However, those values will still be cast to Go `string`s, and they will be converted to `str`
types when re-encoded. It is the responsibility of the user to ensure that map keys are UTF-8 safe in this case.) The same rules hold true
for JSON translation.

If the output compiles, then there's a pretty good chance things are fine. (Plus, we generate tests for you.) Please file an issue if you
think the generator is writing broken code.

### Performance

If you like benchmarks, see [here](https://github.com/dchenk/messagepack-benchmarks), [here](http://bravenewgeek.com/so-you-wanna-go-fast/),
and [here](https://github.com/alecthomas/go_serialization_benchmarks).

## Credits

This repository is a fork of github.com/tinylib/msgp.

Differences between this tool and tinylib/msgp:
- Here we have [regular expression matching](https://github.com/dchenk/msgp/wiki/Using-the-Code-Generator#matching-type-names) for type
names in directives.
- Here we do not use package `unsafe` for conversions from byte slices to strings: `[]byte` is converted quite efficiently to `string`
simply with the built-in `string()`.
- This codebase is thoroughly refactored to be more Go-idiomatic and efficient.

You're welcome to contribute!
