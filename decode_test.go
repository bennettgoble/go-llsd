package llsd

import (
	"bytes"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"testing"
)

type mockTokenReader struct {
	tokens []Token
	offset int64
}

func errorContains(got error, want string) bool {
	if got == nil {
		return want == ""
	}
	if want == "" {
		return false
	}
	return strings.Contains(got.Error(), want)
}

func sInt(i int) Scalar {
	return Scalar{
		Type: Integer,
		Data: []byte(strconv.Itoa(i)),
	}
}

func sBinary(b []byte) Scalar {
	str := hex.EncodeToString(b)
	return Scalar{
		Type: Binary,
		Data: []byte(str),
	}
}

func (r *mockTokenReader) Token() (Token, error) {
	if len(r.tokens) > 0 {
		tok := r.tokens[0]
		err, ok := tok.(error)
		r.tokens = r.tokens[1:]
		r.offset++
		if ok {
			return nil, err
		}
		return tok, nil
	} else {
		return nil, io.EOF
	}
}

func (r *mockTokenReader) Offset() int64 {
	return r.offset
}

func newMockDecoder(tokens ...Token) *Unmarshaler {
	r := &mockTokenReader{tokens: tokens}
	return &Unmarshaler{scan: r, tok: nil, dec: &textDecoder{}, text: true}
}

func TestEOF(t *testing.T) {
	var res struct{}
	dec := newMockDecoder()
	if dec.Unmarshal(&res) != io.EOF {
		t.Fatalf("Expected EOF")
	}
}

func TestToken(t *testing.T) {
	dec := newMockDecoder(1)
	tok, err := dec.token()
	if err != nil {
		t.Fatal(err)
	}
	if tok != 1 {
		t.Fatalf("Expected token to equal 1, got %s", tok)
	}
}

func TestNext(t *testing.T) {
	dec := newMockDecoder(1)
	err := dec.next()
	if err != nil {
		t.Fatal(err)
	}
	if dec.tok != 1 {
		t.Fatalf("Expected token to equal 1, got %s", dec.tok)
	}
}

func TestNoStartDocument(t *testing.T) {
	var dst struct{}
	dec := newMockDecoder(1)
	err := dec.Unmarshal(&dst)
	if !errorContains(err, "Invalid LLSD: missing document start.") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInvalidKey(t *testing.T) {
	var dst string
	dec := newMockDecoder(DocumentStart{}, Key("a"), DocumentEnd{})
	err := dec.Unmarshal(&dst)
	if !errorContains(err, "Invalid LLSD: unexpected Key") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInvalidMap(t *testing.T) {
	var dst struct{ A string }
	dec := newMockDecoder(DocumentStart{}, MapStart{}, Key("A"), MapEnd{}, DocumentEnd{})
	err := dec.Unmarshal(&dst)
	if !errorContains(err, "Invalid LLSD: unexpected MapEnd") {
		t.Errorf("unexpected error: %v", err)
	}

	dec = newMockDecoder(DocumentStart{}, MapStart{}, Key("A"), Key("B"), MapEnd{}, DocumentEnd{})
	err = dec.Unmarshal(&dst)
	if !errorContains(err, "Invalid LLSD: unexpected Key") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTruncateArray(t *testing.T) {
	dst := [2]any{}
	dec := newMockDecoder(DocumentStart{}, ArrayStart{}, sInt(1), sBinary([]byte("Binary data")), sInt(2), ArrayEnd{}, DocumentEnd{})
	if err := dec.Unmarshal(&dst); err != nil {
		t.Fatal(err)
	}
	if dst[0].(int32) != 1 {
		t.Fatalf("Expected dst[0] to equal %d but got %v", 1, dst[0])
	}
	if !bytes.Equal(dst[1].([]byte), []byte("Binary data")) {
		t.Fatalf("Expected dst[1] to equal \"Binary data\" but got %v", dst[1])
	}
}
