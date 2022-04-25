package llsd

import (
	"encoding/ascii85"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type ScalarType int

const (
	Base16 = "base16"
	Base64 = "base64"
	Base85 = "base85"
)

const (
	Undefined ScalarType = iota
	Boolean
	Integer
	Real
	UUIDType
	String
	Binary
	Date
	URI
)

func (t ScalarType) String() string {
	switch t {
	case Undefined:
		return "undef"
	case Boolean:
		return "boolean"
	case Integer:
		return "integer"
	case Real:
		return "real"
	case UUIDType:
		return "uuid"
	case String:
		return "string"
	case Binary:
		return "binary"
	case Date:
		return "date"
	case URI:
		return "uri"
	default:
		return strconv.Itoa(int(t))
	}
}

type ArrayStart struct{}
type ArrayEnd struct{}
type MapStart struct{}
type MapEnd struct{}
type Token any
type Scalar struct {
	Type ScalarType
	Data []byte
	Attr map[string]string
}

type UUID [16]byte
type Key string
type URL string

func (u UUID) String() string {
	return hex.EncodeToString(u[:])
}

type TokenReader interface {
	Token() (Token, error) // Get next LLSD token
	Offset() int64         // Input stream offset
}

type scalarDecoder interface {
	real([]byte) (float64, error)
	uuid([]byte) (UUID, error)
	integer([]byte) (int64, error)
	binary([]byte, string) ([]byte, error)
	date([]byte) (time.Time, error)
	boolean([]byte) (bool, error)
}

type textDecoder struct{}

func (d *textDecoder) real(c []byte) (float64, error) {
	// Default value = 0.0
	if len(c) == 0 || c == nil {
		return 0.0, nil
	}
	f, err := strconv.ParseFloat(string(c), 64)
	if err != nil {
		return 0, err
	}
	return f, nil
}

func (d *textDecoder) uuid(c []byte) (UUID, error) {
	// Default value = 00000000-00000000-00000000-00000000
	if len(c) == 0 || c == nil {
		return [16]byte{}, nil
	}
	h, err := hex.DecodeString(strings.ReplaceAll(string(c), "-", ""))
	if err != nil {
		return [16]byte{}, err
	}
	var u UUID
	copy(u[:], h[:16])
	return u, nil
}

func (d *textDecoder) integer(c []byte) (int64, error) {
	i, err := strconv.Atoi(string(c))
	return int64(i), err
}

func (d *textDecoder) binary(c []byte, encoding string) ([]byte, error) {
	if len(c) == 0 || c == nil {
		return c, nil
	}
	switch encoding {
	case Base16, "":
		dst := make([]byte, hex.DecodedLen(len(c)))
		_, err := hex.Decode(dst, c)
		return dst, err
	case Base64:
		dst := make([]byte, base64.StdEncoding.DecodedLen(len(c)))
		_, err := base64.StdEncoding.Decode(dst, c)
		return dst, err
	case Base85:
		dst := make([]byte, ascii85.MaxEncodedLen(len(c)))
		_, _, err := ascii85.Decode(dst, c, true)
		return dst, err
	default:
		return nil, errors.New(fmt.Sprintf("Unknown encoding \"%s\"", encoding))
	}
}

func (d *textDecoder) boolean(c []byte) (bool, error) {
	if len(c) == 0 || c == nil {
		return false, nil
	}
	if string(c) == "1" || string(c) == "true" {
		return true, nil
	} else if string(c) == "0" || string(c) == "false" {
		return false, nil
	}
	return false, errors.New(fmt.Sprintf("Invalid boolean value %s", c))
}

func (d *textDecoder) date(c []byte) (time.Time, error) {
	if len(c) == 0 || c == nil {
		return time.Unix(0, 0), nil
	}
	return time.Parse(time.RFC3339, string(c))
}

type binaryDecoder struct{}

func (d *binaryDecoder) real(b []byte) (float64, error) {
	// Default value = 0.0
	if len(b) == 0 || b == nil {
		return 0.0, nil
	}
	bits := binary.BigEndian.Uint64(b)
	return math.Float64frombits(bits), nil
}

func (d *binaryDecoder) uuid(b []byte) (UUID, error) {
	// Default value = 00000000-00000000-00000000-00000000
	if len(b) == 0 || b == nil {
		return [16]byte{}, nil
	}
	var u UUID
	copy(u[:], b[:16])
	return u, nil
}

func (d *binaryDecoder) integer(b []byte) (int64, error) {
	return int64(binary.BigEndian.Uint32(b)), nil
}

func (d *binaryDecoder) binary(b []byte, encoding string) ([]byte, error) {
	return b, nil
}

func (d *binaryDecoder) boolean(b []byte) (bool, error) {
	return len(b) > 1, nil
}

func (d *binaryDecoder) date(b []byte) (time.Time, error) {
	epoch, err := d.integer(b)
	if err != nil {
		return time.Unix(0, 0), err
	}
	return time.Unix(epoch, 0), nil
}
