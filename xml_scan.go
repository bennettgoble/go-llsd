package llsd

import (
	"encoding/xml"
	"fmt"
	"io"
	"reflect"
)

type XMLScanner struct {
	dec *xml.Decoder
}

func NewXMLScanner(r io.Reader) *XMLScanner {
	return &XMLScanner{dec: xml.NewDecoder(r)}
}

// InputOffset returns the input stream byte offset of the current decoder position.
func (s *XMLScanner) Offset() int64 {
	return s.dec.InputOffset()
}

func (s *XMLScanner) charData() ([]byte, error) {
	// Attempt to get CharData (inner-text)
	t, err := s.dec.Token()
	if err != nil {
		return nil, err
	}
	switch ty := t.(type) {
	case xml.CharData:
		return ty, nil
	case xml.EndElement:
		// Handle self-closing elements
		return nil, nil
	default:
		return nil, fmt.Errorf("Invalid LLSD: got unexpected %s", reflect.TypeOf(t))
	}
}

// Skip element, useful for jumping over large maps and arrays.
func (s *XMLScanner) Skip() error {
	return s.dec.Skip()
}

func (s *XMLScanner) Token() (Token, error) {
	tok, err := s.dec.Token()

	if err != nil {
		return nil, err
	}

	switch ty := tok.(type) {
	case xml.StartElement:
		switch ty.Name.Local {
		case "array":
			return ArrayStart{}, nil
		case "map":
			return MapStart{}, nil
		case "key":
			b, err := s.charData()
			key := Key(b)
			if err != nil {
				return key, err
			}
			// Advanced past </key> EndElement, which is always provided by go's xml decoding
			_, err = s.dec.Token()

			return key, err
		case "llsd":
			// Skip document start
			return s.Token()
		default:
			scalarTypes := map[string]ScalarType{
				"string":  String,
				"real":    Real,
				"uuid":    UUIDType,
				"integer": Integer,
				"boolean": Boolean,
				"undef":   Undefined,
				"binary":  Binary,
				"uri":     URI,
				"date":    Date,
			}

			scalarType, ok := scalarTypes[ty.Name.Local]

			if !ok {
				return nil, fmt.Errorf("Unknown LLSD type \"%s\"", ty.Name.Local)
			}

			// Copy data so that it is not overwritten when advancing past end element
			innerText, err := s.charData()
			data := make([]byte, len(innerText))
			copy(data, innerText)

			// Map XML attributes (<binary encoding="base64">)
			attr := map[string]string{}
			for _, a := range ty.Attr {
				attr[a.Name.Local] = a.Value
			}

			if err != nil {
				return nil, err
			}

			// If innerText is nil then the element is self-closing and we have
			// already advanced past its EndElement.
			if innerText != nil {
				// Advanced past EndElement, which is always provided by go's xml decoding
				_, err = s.dec.Token()
			}

			return Scalar{Type: scalarType, Data: data, Attr: attr}, err
		}
	case xml.EndElement:
		switch ty.Name.Local {
		case "array":
			return ArrayEnd{}, nil
		case "map":
			return MapEnd{}, nil
		case "llsd":
			return s.Token()
		default:
			return nil, fmt.Errorf("Invalid LLSD: unexpected EndElement %s", ty.Name.Local)
		}
	case xml.Comment, xml.ProcInst, xml.CharData:
		// Skip comments, (<!-- ... -->)
		// Skip XML processing instructions, (<xml ... >)
		// Skip character data between elements such as whitespace
		return s.Token()
	default:
		return nil, fmt.Errorf("Invalid LLSD. Unexpected %s at %d", reflect.TypeOf(tok), s.Offset())
	}
}
