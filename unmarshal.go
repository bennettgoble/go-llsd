package llsd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"
)

// MarshalTypeError represents an error in the marshaling process.
type MarshalTypeError struct {
	Type reflect.Type
}

func (e *MarshalTypeError) Error() string {
	return "LLSD: Cannot marshal Go value of type " + e.Type.String() + "."
}

// UnmarshalTypeError represents an error in the unmarshaling process.
type UnmarshalTypeError struct {
	Value  string       // Description of LLSD value - "real", "map", "string"
	Type   reflect.Type // Type of Go value that could not be assigned to
	Offset int64        // Input stream byte offset where error occurred
}

func (e *UnmarshalTypeError) Error() string {
	return "LLSD: Cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String() + "."
}

// InvalidLLSDError represents a problem with input LLSD.
type InvalidLLSDError struct {
	Problem string
	Offset  int64
}

func (e *InvalidLLSDError) Error() string {
	return "Invalid LLSD: " + e.Problem
}

// Decoder is a generic LLSD unmarshaler that can work with any TokenReader.
type Unmarshaler struct {
	DisallowUnknownFields bool
	text                  bool // whether decoding text (notation, xml) or binary llsd
	dec                   scalarDecoder
	scan                  TokenReader
	tok                   Token // last read token
}

// TextUnmarshaler is the interface implemented by types that want to
// customize how text (xml, notation) LLSD values are unmarshaled into
// themselves.
type TextUnmarshaler interface {
	UnmarshalTextLLSD([]byte) error
}

// BinaryUnmarshaler is the interface implemented by types that want to
// customize how binary LLSD values are unmarshaled into themselves.
type BinaryUnmarshaler interface {
	UnmarshalBinaryLLSD([]byte) error
}

// TextMarshaler is the interface implemented by types that want to
// customize how text (xml, notation) LLSD values are marshaled into
// text.
type TextMarshaler interface {
	MarshalTextLLSD() (ScalarType, string, error)
}

// TextMarshaler is the interface implemented by types that want to
// customize how text (xml, notation) LLSD values are marshaled into
// text.
type BinaryMarshaler interface {
	MarshalBinaryLLSD() (ScalarType, []byte, error)
}

// Unmarshal decodes LLSD into a given value.
func (u *Unmarshaler) Unmarshal(v any) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Pointer {
		return errors.New("Non-pointer passed to Unmarshal")
	}

	// Read first value
	if err := u.next(); err != nil {
		return err
	}

	return u.value(val)
}

// token advances the parser to the next token and returns its value.
func (u *Unmarshaler) token() (Token, error) {
	tok, err := u.scan.Token()
	u.tok = tok
	return tok, err
}

// next advances the parser to the next token.
func (u *Unmarshaler) next() error {
	tok, err := u.scan.Token()
	u.tok = tok
	return err
}

// value unmarshals a single value.
func (u *Unmarshaler) value(v reflect.Value) error {
	switch u.tok.(type) {
	case MapStart:
		if v.IsValid() {
			if err := u.object(v); err != nil {
				return err
			}
		}
	case ArrayStart:
		if v.IsValid() {
			if err := u.array(v); err != nil {
				return err
			}
		}
	case Scalar:
		if v.IsValid() {
			if err := u.scalar(v); err != nil {
				return err
			}
		}
	default:
		return &InvalidLLSDError{Problem: fmt.Sprintf("unexpected %s", reflect.TypeOf(u.tok).Name()), Offset: u.scan.Offset()}
	}
	return nil
}

// tag stores information parsed from the llsd field tag.
type tag struct {
	Encoding  string // Binary field text encoding, base16, base64, base85
	Name      string // Override Go member name `llsd:"name"`
	Omit      bool
	OmitEmpty bool
}

// parseTag parses a llsd or json field tag.
func parseTag(t, name string) tag {
	if t == "" {
		return tag{Name: name}
	}
	values := strings.Split(t, ",")
	if t == "-" {
		return tag{Omit: true, Name: name}
	}
	if values[0] != "" {
		name = values[0]
	}
	omitEmpty := false
	encoding := Base16
	if len(values) > 1 {
		for _, v := range values[1:] {
			switch v {
			case "omitempty":
				omitEmpty = true
			case Base16, Base64, Base85:
				encoding = v
			}
		}
	}
	return tag{
		Name:      name,
		OmitEmpty: omitEmpty,
		Encoding:  encoding,
	}
}

type fieldInfo struct {
	reflect.StructField
	LLSDTag tag
}

type fieldInfoMap map[string]fieldInfo

// fieldsForType collects field information from structs, parsing llsd/json tag information
// for use during deserialization/serialization
func fieldsForType(t reflect.Type) fieldInfoMap {
	fields := fieldInfoMap{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		tagStr := field.Tag.Get("llsd")
		if tagStr == "" {
			tagStr = field.Tag.Get("json")
		}

		tag := parseTag(tagStr, field.Name)
		fields[tag.Name] = fieldInfo{field, tag}
	}
	return fields
}

var fieldCache sync.Map // map[reflect.Type]fieldInfo

// cachedFieldsForType retrieves cached field information of a type or constructs it if not found
func cachedFieldsForType(t reflect.Type) fieldInfoMap {
	if f, ok := fieldCache.Load(t); ok {
		return f.(fieldInfoMap)
	}
	f, _ := fieldCache.LoadOrStore(t, fieldsForType(t))
	return f.(fieldInfoMap)
}

// Unmarshal an object.
func (u *Unmarshaler) object(v reflect.Value) error {

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		fields := cachedFieldsForType(v.Type())

		for {
			// Read next key
			var key string
			tok, err := u.token()
			if err != nil {
				return err
			}

			switch tok.(type) {
			case Key:
				key = string(tok.(Key))
			case MapEnd:
				// Done reading object
				return nil
			default:
				return &InvalidLLSDError{Problem: fmt.Sprintf("expected map to start with key, got %s", reflect.TypeOf(tok).Name()), Offset: u.scan.Offset()}
			}

			// Find field cooresponding to key
			field, ok := fields[key]
			if !ok {
				if u.DisallowUnknownFields {
					return errors.New(fmt.Sprintf("LLSD: Unknown field %q", key))
				}
				// Skip unknown field (And possibly skip past invalid JSON...)
				if err = u.next(); err != nil {
					return err
				}
				continue
			}

			// Advance to presumed value and use it
			if err = u.next(); err != nil {
				return err
			}
			subv := v.FieldByIndex(field.Index)
			if err = u.value(subv); err != nil {
				return err
			}
		}
	case reflect.Map:
		ty := v.Type()
		kType := ty.Key()
		vType := ty.Elem()
		if kType.Kind() != reflect.String {
			return &UnmarshalTypeError{Value: "map ", Type: ty, Offset: u.scan.Offset()}
		}
		for {
			// Read next key
			var key string
			tok, err := u.token()
			if err != nil {
				return err
			}

			switch tok.(type) {
			case Key:
				key = string(tok.(Key))
			case MapEnd:
				// Done reading object
				return nil
			default:
				return &InvalidLLSDError{Problem: fmt.Sprintf("expected map to start with key, got %s", reflect.TypeOf(tok).Name()), Offset: u.scan.Offset()}
			}

			// Advance to presumed value and use it
			subv := reflect.New(vType).Elem()
			if err = u.next(); err != nil {
				return err
			}
			if err = u.value(subv); err != nil {
				return err
			}
			v.SetMapIndex(reflect.ValueOf(key), subv)
		}
	default:
		return &UnmarshalTypeError{Value: "object", Type: v.Type(), Offset: u.scan.Offset()}
	}
}

func (u *Unmarshaler) array(v reflect.Value) error {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Interface:
		if v.NumMethod() == 0 {
			// Decode into nil interface
			newv := reflect.ValueOf([]any{})
			v.Set(newv)
			if err := u.array(newv); err != nil {
				return err
			}
		}
		fallthrough
	case reflect.Slice, reflect.Array:
		i := 0
		for {
			// Read next value
			tok, err := u.token()
			if err != nil {
				return err
			}

			switch tok.(type) {
			case ArrayEnd:
				// Done reading array
				return nil
			}

			// grow slice
			if v.Kind() == reflect.Slice {
				if i >= v.Cap() {
					newcap := v.Cap() + v.Cap()/2
					if newcap < 4 {
						newcap = 4
					}
					newv := reflect.MakeSlice(v.Type(), v.Len(), newcap)
					reflect.Copy(newv, v)
					v.Set(newv)
				}
				if i >= v.Len() {
					v.SetLen(i + 1)
				}
			}

			if i < v.Len() {
				// Decode into value
				if err := u.value(v.Index(i)); err != nil {
					return err
				}
			} else {
				// Skip remaining elements (fixed array)
				if err := u.value(reflect.Value{}); err != nil {
					return err
				}
			}
			i++
		}
	default:
		return &UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: u.scan.Offset()}
	}
}

func (u *Unmarshaler) scalar(v reflect.Value) error {
	// Use custom unmarshaler if present
	tok := u.tok.(Scalar)
	if u.text {
		un, ok := v.Interface().(TextUnmarshaler)
		if ok {
			return un.UnmarshalTextLLSD(tok.Data)
		}
	} else {
		un, ok := v.Interface().(BinaryUnmarshaler)
		if ok {
			return un.UnmarshalBinaryLLSD(tok.Data)
		}
	}

	if v.Kind() == reflect.Pointer {
		// Allow <undef /> to result in a null pointer
		if tok.Type == Undefined {
			return nil
		}
		// Initialize with default value
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch tok.Type {
	case Real:
		switch v.Kind() {
		case reflect.Float32, reflect.Float64:
			value, err := u.dec.real(tok.Data)
			if err != nil {
				return err
			}
			if v.OverflowFloat(value) {
				return &UnmarshalTypeError{Value: "real " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			v.SetFloat(value)
		case reflect.Interface:
			value, err := u.dec.real(tok.Data)
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(value))
		default:
			return &UnmarshalTypeError{Value: "real " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
		}
	case Integer:
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value, err := u.dec.integer(tok.Data)
			if err != nil {
				return err
			}
			if v.OverflowInt(value) {
				return &UnmarshalTypeError{Value: "integer " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			v.SetInt(value)
		case reflect.Interface:
			value, err := u.dec.integer(tok.Data)
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(int32(value)))
		default:
			return &UnmarshalTypeError{Value: "integer " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
		}
	case URI:
		v.Set(reflect.ValueOf(URL(tok.Data)))
	case String:
		switch v.Kind() {
		case reflect.String, reflect.Interface:
			v.Set(reflect.ValueOf(string(tok.Data)))
		default:
			return &UnmarshalTypeError{Value: "string " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
		}
	case Boolean:
		switch v.Kind() {
		case reflect.Bool, reflect.Interface:
			value, err := u.dec.boolean(tok.Data)
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(value))
		case reflect.String:
			value, err := u.dec.boolean(tok.Data)
			if err != nil {
				return err
			}
			if value {
				v.SetString("true")
			} else {
				v.SetString("false")
			}
		default:
			return &UnmarshalTypeError{Value: "boolean " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
		}
	case Binary:
		encoding := ""
		if u.text {
			// Handle possible text encodings: base16, base64, base85
			ok := false
			encoding, ok = tok.Attr["encoding"]
			if !ok {
				encoding = Base16
			}
		}
		value, err := u.dec.binary(tok.Data, encoding)
		if err != nil {
			return err
		}
		// Support some of the hare-brained conversions for binary specified at
		// https://wiki.secondlife.com/wiki/LLSD#Conversion_6
		switch v.Kind() {
		case reflect.Slice, reflect.Array:
			if v.Type().Elem().Kind() != reflect.Uint8 {
				return &UnmarshalTypeError{Value: "binary " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			if v.Kind() == reflect.Array {
				reflect.Copy(v, reflect.ValueOf(value))
			} else {
				v.Set(reflect.ValueOf(value))
			}
		case reflect.Interface:
			v.Set(reflect.ValueOf(value))
		case reflect.String:
			v.SetString(string(value))
		case reflect.Bool:
			v.SetBool(len(tok.Data) > 0)
		case reflect.Uint, reflect.Uint32:
			if len(value) < 4 {
				return &UnmarshalTypeError{Value: "binary (too few bytes) " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			bits := binary.BigEndian.Uint32(value[:4])
			v.SetUint(uint64(bits))
		case reflect.Float32:
			bits := binary.BigEndian.Uint32(value[:4])
			f := math.Float32frombits(bits)
			v.SetFloat(float64(f))
		case reflect.Int, reflect.Int32:
			if len(value) < 4 {
				return &UnmarshalTypeError{Value: "binary (too few bytes) " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			bits := binary.BigEndian.Uint32(value[:4])
			v.SetInt(int64(bits))
		case reflect.Int64:
			if len(value) < 8 {
				return &UnmarshalTypeError{Value: "binary (too few bytes) " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			bits := binary.BigEndian.Uint64(value[:8])
			v.SetInt(int64(bits))
		case reflect.Uint64:
			if len(value) < 8 {
				return &UnmarshalTypeError{Value: "binary (too few bytes) " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			bits := binary.BigEndian.Uint64(value[:8])
			v.SetUint(bits)
		case reflect.Float64:
			if len(value) < 8 {
				return &UnmarshalTypeError{Value: "binary (too few bytes) " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			bits := binary.BigEndian.Uint64(value[:8])
			f := math.Float64frombits(bits)
			v.SetFloat(f)
		default:
			return &UnmarshalTypeError{Value: "binary " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
		}
	case Date:
		switch v.Kind() {
		case reflect.Float32, reflect.Float64:
			value, err := u.dec.date(tok.Data)
			if err != nil {
				return err
			}
			epoch := float64(value.Unix())
			if v.OverflowFloat(epoch) {
				return &UnmarshalTypeError{Value: "date " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			v.SetFloat(epoch)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value, err := u.dec.date(tok.Data)
			if err != nil {
				return err
			}
			epoch := value.Unix()
			if v.OverflowInt(epoch) {
				return &UnmarshalTypeError{Value: "date " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			v.SetInt(epoch)
		case reflect.String:
			v.SetString(string(tok.Data))
		case reflect.Interface:
			value, err := u.dec.date(tok.Data)
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(value))
		default:
			if _, ok := v.Interface().(time.Time); !ok {
				return &UnmarshalTypeError{Value: "date " + string(tok.Data), Type: v.Type(), Offset: u.scan.Offset()}
			}
			value, err := u.dec.date(tok.Data)
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(value))
		}
	case Undefined:
		// ok?
		return nil
	}
	return nil
}

// UnmarshalXML attempts to deserialize given LLSD XML data into a given value.
func UnmarshalXML(data []byte, v any) error {
	return NewXMLDecoder(bytes.NewReader(data)).Unmarshal(v)
}

// UnmarshalBinary attempts to deserialize given LLSD binary data into a given value.
func UnmarshalBinary(data []byte, v any) error {
	return NewBinaryDecoder(bytes.NewReader(data)).Unmarshal(v)
}

// NewXMLDecoder creates a new instance of an Unmarshaler configured to read LLSD XML.
func NewXMLDecoder(r io.Reader) *Unmarshaler {
	return &Unmarshaler{scan: NewXMLScanner(r), tok: nil, dec: &textDecoder{}, text: true}
}

// NewBinaryDecoder creates a new instance of an Unmarshaler configured to read binary LLSD.
func NewBinaryDecoder(r io.Reader) *Unmarshaler {
	return &Unmarshaler{scan: NewBinaryScanner(r), tok: nil, dec: &binaryDecoder{}, text: false}
}
