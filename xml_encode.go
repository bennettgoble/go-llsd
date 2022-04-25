package llsd

import (
	"bufio"
	"bytes"
	"encoding/ascii85"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type XMLEncoder struct {
	w      *bufio.Writer
	indent string
	depth  int
}

func MarshalXML(v any) ([]byte, error) {
	var b bytes.Buffer
	if err := NewXMLEncoder(&b).Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func MarshalXMLIndent(v any, indent string) ([]byte, error) {
	var b bytes.Buffer
	enc := NewXMLEncoder(&b)
	enc.SetIndent(indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func NewXMLEncoder(w io.Writer) *XMLEncoder {
	return &XMLEncoder{w: bufio.NewWriter(w)}
}

func (e *XMLEncoder) writeIndent() {
	if e.indent == "" {
		return
	}
	e.writeString("\n" + strings.Repeat(e.indent, e.depth))
}

func (e *XMLEncoder) Encode(v any) error {
	e.writeString(xml.Header)
	e.writeString("<llsd>")
	e.depth++
	err := e.marshalValue(reflect.ValueOf(v), nil)
	if err != nil {
		return err
	}
	e.depth--
	e.writeIndent()
	e.writeString("</llsd>")
	e.Flush()
	return nil
}

func (c *XMLEncoder) marshalValue(v reflect.Value, info *fieldInfo) error {

	// Skip unexported fields
	if !v.CanInterface() {
		return nil
	}

	if !v.IsValid() {
		return nil
	}

	if info != nil && info.LLSDTag.OmitEmpty && isEmptyValue(v) {
		return nil
	}

	// Use custom marshaler
	m, ok := v.Interface().(TextMarshaler)
	if ok {
		ty, val, err := m.MarshalTextLLSD()
		if err != nil {
			return err
		}
		c.writeIndent()
		c.writeString(fmt.Sprintf("<%s>%s</%s>", ty, val, ty))
		return nil
	}

	if v.Kind() == reflect.Pointer {
		// Write null pointer as Undef
		if v.IsNil() {
			c.writeIndent()
			c.writeString("<undef />")
			return nil
		}
		// If not a null pointer then get the actual value
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Interface:
		return c.marshalValue(v.Elem(), nil)
	case reflect.Struct:
		c.writeIndent()
		c.writeString("<map>")
		c.depth++
		fields := cachedFieldsForType(v.Type())
		for key, field := range fields {
			if field.LLSDTag.Omit {
				continue
			}
			subv := v.FieldByIndex(field.Index)
			// Skip unexported fields
			if !subv.CanInterface() {
				continue
			}
			if field.LLSDTag.OmitEmpty && isEmptyValue(subv) {
				continue
			}
			c.writeIndent()
			c.writeString("<key>")
			c.writeString(key)
			c.writeString("</key>")
			if err := c.marshalValue(subv, &field); err != nil {
				return err
			}
		}
		c.depth--
		c.writeIndent()
		c.writeString("</map>")
	case reflect.Map:
		c.writeIndent()
		c.writeString("<map>")
		c.depth++
		for _, key := range v.MapKeys() {
			c.writeIndent()
			subv := v.MapIndex(key)
			// Skip unexported fields
			if !subv.CanInterface() {
				continue
			}
			c.writeString("<key>")
			// TODO: Make key marshaling more flexible
			if err := xml.EscapeText(c.w, []byte(key.String())); err != nil {
				return err
			}
			c.writeString("</key>")
			if err := c.marshalValue(subv, nil); err != nil {
				return err
			}
		}
		c.writeIndent()
		c.writeString("</map>")
		c.depth--
	case reflect.Array, reflect.Slice:
		// There has to be a better way of getting reflect.Type of byte
		if v.Type().Elem().Kind() == reflect.Uint8 {
			c.writeIndent()
			encoding := Base16
			if info != nil && info.LLSDTag.Encoding != "" {
				encoding = info.LLSDTag.Encoding
			}
			if encoding == Base16 {
				c.writeString("<binary>")
			} else {
				c.writeString(fmt.Sprintf("<binary encoding=\"%s\">", encoding))
			}
			slice, ok := v.Slice(0, v.Len()).Interface().([]byte)
			if !ok {
				return errors.New("Unable to cast binary slice")
			}
			if err := c.writeBytes(slice, encoding); err != nil {
				return err
			}
			c.writeString("</binary>")
			return nil
		}
		c.writeIndent()
		c.writeString("<array>")
		c.depth++
		for i := 0; i < v.Len(); i++ {
			subv := v.Index(i)
			if err := c.marshalValue(subv, nil); err != nil {
				return err
			}
		}
		c.writeString("</array>")
		c.depth--
	case reflect.String:
		if _, ok := v.Interface().(URL); ok {
			c.writeIndent()
			c.writeString("<uri>")
			if err := xml.EscapeText(c.w, []byte(v.String())); err != nil {
				return err
			}
			c.writeString("</uri>")
		} else {
			c.writeIndent()
			c.writeString("<string>")
			if err := xml.EscapeText(c.w, []byte(v.String())); err != nil {
				return err
			}
			c.writeString("</string>")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		c.writeIndent()
		c.writeString("<integer>")
		c.writeString(strconv.FormatInt(v.Int(), 10))
		c.writeString("</integer>")
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		c.writeIndent()
		c.writeString("<integer>")
		c.writeString(strconv.FormatUint(v.Uint(), 10))
		c.writeString("</integer>")
	case reflect.Float32, reflect.Float64:
		c.writeIndent()
		c.writeString("<real>")
		c.writeString(fmt.Sprintf("%f", v.Float()))
		c.writeString("</real>")
	case reflect.Bool:
		c.writeIndent()
		c.writeString("<boolean>")
		if v.Bool() {
			c.writeString("1")
		} else {
			c.writeString("0")
		}
		c.writeString("</boolean>")
	default:
		vi := v.Interface()
		switch vi.(type) {
		case UUID:
			c.writeIndent()
			c.writeString("<uuid>")
			u, ok := vi.(UUID)
			if !ok {
				return errors.New("Unable to cast UUID value")
			}
			c.writeString(u.String())
			c.writeString("</uuid>")
		case URL:
			c.writeIndent()
			c.writeString("<uri>")
			if err := xml.EscapeText(c.w, []byte(v.String())); err != nil {
				return err
			}
			c.writeString("</uri>")
		case url.URL:
			c.writeIndent()
			c.writeString("<uri>")
			url, ok := vi.(url.URL)
			if !ok {
				return errors.New("Unable to cast url.URL value")
			}
			if err := xml.EscapeText(c.w, []byte(url.String())); err != nil {
				return err
			}
			c.writeString("</uri>")
		case time.Time:
			c.writeIndent()
			c.writeString("<date>")
			t, ok := vi.(time.Time)
			if !ok {
				return errors.New("Unable to cast time.Time value")
			}
			c.writeString(t.Format(time.RFC3339))
			c.writeString("</date>")
		default:
			return &MarshalTypeError{Type: v.Type()}
		}
	}
	return nil
}

func (e *XMLEncoder) writeBytes(b []byte, encoding string) error {
	switch encoding {
	case Base16:
		// write upper case as the llbase python module expects it
		e.writeString(strings.ToUpper(hex.EncodeToString(b)))
	case Base64:
		e.writeString(base64.StdEncoding.EncodeToString(b))
	case Base85:
		dst := make([]byte, ascii85.MaxEncodedLen(len(b)))
		ascii85.Encode(dst, b)
		if err := xml.EscapeText(e.w, dst); err != nil {
			return err
		}
	default:
		return errors.New("Unknown encoding " + encoding)
	}
	return nil
}

func (e *XMLEncoder) writeString(s string) {
	_, _ = e.w.WriteString(s)
}

func (e *XMLEncoder) SetIndent(indent string) {
	e.indent = indent
}

// Flush flushes any buffered XML to the underlying writer
func (e *XMLEncoder) Flush() {
	e.w.Flush()
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}
