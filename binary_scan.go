package llsd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const BinaryHeader = "<?llsd/binary?>\n"

type BinaryScanner struct {
	r   io.Reader
	off int64
}

func NewBinaryScanner(r io.Reader) *BinaryScanner {
	return &BinaryScanner{r: r}
}

func (s *BinaryScanner) Offset() int64 {
	return s.off
}

func (s *BinaryScanner) Token() (Token, error) {
	for {
		op, err := s.read(1)
		if err != nil {
			return nil, err
		}
		switch op[0] {
		case 'i':
			buf, err := s.read(4)
			if err != nil {
				return nil, err
			}
			return Scalar{Type: Integer, Data: buf}, nil
		case 'r':
			buf, err := s.read(8)
			return Scalar{Type: Real, Data: buf}, err
		case 'u':
			buf, err := s.read(16)
			return Scalar{Type: UUIDType, Data: buf}, err
		case 'b':
			buf, err := s.read(4)
			if err != nil {
				return nil, err
			}
			size := binary.BigEndian.Uint32(buf)
			buf, err = s.read(size)
			return Scalar{Type: Binary, Data: buf}, err
		case 's':
			buf, err := s.read(4)
			if err != nil {
				return nil, err
			}
			size := binary.BigEndian.Uint32(buf)
			buf, err = s.read(size)
			return Scalar{Type: String, Data: buf}, err
		case 'd':
			buf, err := s.read(4)
			return Scalar{Type: Date, Data: buf}, err
		case 'k':
			buf, err := s.read(4)
			if err != nil {
				return nil, err
			}
			size := binary.BigEndian.Uint32(buf)
			buf, err = s.read(size)
			return Key(buf), err
		case '{':
			// Eat map size, could use it to provide a skip() method
			_, err := s.read(4)
			return MapStart{}, err
		case '}':
			return MapEnd{}, nil
		case '[':
			// Eat array size
			_, err := s.read(4)
			return ArrayStart{}, err
		case ']':
			return ArrayEnd{}, nil
		case '1':
			return Scalar{Type: Boolean, Data: []byte{1}}, nil
		case '0':
			return Scalar{Type: Boolean, Data: []byte{}}, nil
		case '!':
			return Scalar{Type: Undefined}, nil
		case '<':
			// Read header
			if s.off == 1 {
				buf, err := s.read(15)
				if err != nil {
					return nil, err
				}
				header := "<" + string(buf)
				if header != BinaryHeader {
					return nil, errors.New(fmt.Sprintf("Invalid LLSD: unrecognized header %s", header))
				}
				continue
			}
			fallthrough
		default:
			return nil, errors.New(fmt.Sprintf("Invalid LLSD %s", op))
		}

	}
}

func (s *BinaryScanner) read(num uint32) ([]byte, error) {
	buf := make([]byte, num)
	_, err := s.r.Read(buf)
	s.off += int64(num)
	if err != nil {
		return buf, err
	}
	return buf, nil
}
