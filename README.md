# go-llsd

[LLSD][llsd] encoder/decoder for Go.

## Example use

**go-llsd** exposes an interface similar to Go's json/xml stdlib. Annotate
struct fields with a `llsd` tag to customize property names. 

Basic unmarshaling:
```go
type data struct {
    FieldA string `llsd:"field_a"`
}

const xml = `<?xml version="1.0" encoding="UTF-8"?>
<llsd>
    <map>
      <key>field_a</key><string>Hello, world</string>
    </map>
</llsd>`

var d data
if err := llsd.UnmarshalXML(&d); err != nil {
    panic(err)
}
println(data.FieldA)
```

Basic marshaling:
```go
type data struct {
    FieldA string `llsd:"field_a"`
}
d := data{FieldA: "Hello, world"}

xml, err := llsd.MarshalXML(&d)
if err != nil {
    panic(err)
}
println(string(xml))
```

Full example:
```go
package main

import (
    "fmt"

    "github.com/bennettgoble/go-llsd"
)

type StatsResponse struct {
    RegionID llsd.UUID `llsd:"region_id"`
    Scale    string    `llsd:"scale"`
    Stats    struct {
        TimeDilation float64 `llsd:"time dilation"`
        SimFPS       float64 `llsd:"sim fps"`
        PhysicsFPS   float64 `llsd:"physics fps"`
    } `llsd:"simulator statistics"`
}

func main() {
    const xml = `<?xml version="1.0" encoding="UTF-8"?>
    <llsd>
    <map>
        <key>region_id</key><uuid>67153d5b-3659-afb4-8510-adda2c034649</uuid>
        <key>scale</key><string>one minute</string>
        <key>simulator statistics</key>
        <map>
          <key>time dilation</key><real>0.9878624</real>
          <key>sim fps</key><real>44.9000000</real>
          <key>physics fps</key><real>45.0000000</real>
        </map>
    </map>
    </llsd>`

    var res StatsResponse
    if err := llsd.UnmarshalXML([]byte(xml), &res); err != nil {
        panic(err)
    }
    println(fmt.Sprintf("Sim FPS: %f", res.Stats.SimFPS))
}
```

### Field tags 

Examples of `llsd` struct field tags and their meanings:

```go
// Field appears in LLSD with key "my_name"
Field int `llsd:"my_name"`

// Field appears in LLSD with key "my_name" and is omitted
// from the object if it has zero-value
Field int `llsd:"my_name,omitempty"`

// Field is omitted from object if it has zero-value
Field int `llsd:",omitempty"`

// Field is ignored
Field int `llsd:"-"`

// Field appears in LLSD with key "-"
Field int `llsd:"-,"`

// Field appears in LLSD with key "my_name" and uses
// base64 text representation 
Field []byte `llsd:"my_name,base64"`

// Field uses base64 text representation 
Field []byte `llsd:",base64"`

// Field uses base85 text representation (Don't do this, it's gross)
Field []byte `llsd:",base85"`
```

As a convenience, **go-llsd** will attempt to use `json` [tags][json] if `llsd` is not
specified.

### Custom marshaling/unmarshaling

You may define custom marshaling/unmarshaling behavior for scalar types by
implementing the appropriate Unmarshaler/Marshaler interface.

```go
// TextUnmarshaler is the interface implemented by types that want to
// customize how text (xml, notation) LLSD values are unmarshaled into
// themselves.
type TextUnmarshaler interface {
	UnmarshalTextLLSD([]byte) error
}

// TextMarshaler is the interface implemented by types that want to
// customize how text (xml, notation) LLSD values are marshaled into
// text.
type TextMarshaler interface {
	MarshalTextLLSD() (llsd.ScalarType, string, error)
}
```

For example, to define a type that parses CSV values from LLSD `string`:
```go
type csv []string

func (c *csv) UnmarshalTextLLSD(b []byte) error {
	v := strings.Split(string(b), ",")
	*c = append(*c, v...)
	return nil
}

func (c *csv) MarshalTextLLSD() (llsd.ScalarType, string, error) {
    return llsd.String, strings.Join(*c, ","), nil
}
```

### Binary support

Binary LLSD can be parsed using methods similar to XML:
```go
var dst MyType

err := llsd.UnmarshalBinary(data, &dst)
if err != nil {
    panic(err)
}
```

### Notes on behavior

- Using fixed-length arrays causes extra values to be ignored 
- nullptr is serialized as `undef`

## TODO

- [x] Marshaling
- [x] Add basic benchmarking
- [ ] Parameterize test suite
- [x] Add support for binary
- [ ] Add support for notation 

[llsd]: https://wiki.secondlife.com/wiki/LLSD
[json]: https://pkg.go.dev/encoding/json#Marshal
