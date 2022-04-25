package llsd

import (
	"bytes"
	"testing"
)

func TestTextReal(t *testing.T) {
	d := &textDecoder{}
	for _, c := range []struct {
		val      []byte
		expected float64
	}{
		{val: nil, expected: 0.0},
		{val: []byte(""), expected: 0.0},
		{val: []byte("1.0"), expected: 1.0},
		{val: []byte("-1.0"), expected: -1.0},
		{val: []byte("0.0"), expected: 0.0},
	} {
		got, err := d.real(c.val)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.expected {
			t.Fatalf("Expected %f, got %f", c.expected, got)
		}
	}
}

func TestTextUUID(t *testing.T) {
	d := &textDecoder{}
	for _, c := range []struct {
		val      []byte
		expected string
	}{
		{val: nil, expected: "00000000000000000000000000000000"},
		{val: []byte(""), expected: "00000000000000000000000000000000"},
		{val: []byte("6d1e8348-df64-486b-bf4e-afe049dc3b83"), expected: "6d1e8348df64486bbf4eafe049dc3b83"},
		{val: []byte("6d1e8348df64486bbf4eafe049dc3b83"), expected: "6d1e8348df64486bbf4eafe049dc3b83"},
	} {
		got, err := d.uuid(c.val)
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != c.expected {
			t.Fatalf("Expected %s, got %s", c.expected, got)
		}
	}
}

func TestBinary(t *testing.T) {
	d := textDecoder{}
	for _, c := range []struct {
		val      []byte
		expected string
		encoding string
		err      string
	}{
		{val: nil, expected: ""},
		{val: []byte("42696E6172792064617461"), expected: "Binary data", encoding: ""},
		{val: []byte("42696E6172792064617461"), expected: "Binary data", encoding: "base16"},
		{val: []byte("QmluYXJ5IGRhdGE="), expected: "Binary data", encoding: "base64"},
		{val: []byte("6>:=GEd8d<@<>o"), expected: "Binary data", encoding: "base85"},
		{val: []byte("f"), encoding: "a", err: "Unknown encoding \"a\""},
	} {
		got, err := d.binary(c.val, c.encoding)
		if !errorContains(err, c.err) {
			t.Fatal(err)
		}
		if string(bytes.Trim(got, "\x00")) != c.expected {
			t.Fatalf("Expected %s, got %s", c.expected, got)
		}
	}
}

func TestBoolean(t *testing.T) {
	d := textDecoder{}
	for _, c := range []struct {
		val      []byte
		expected bool
		err      string
	}{
		{val: nil, expected: false},
		{val: []byte("0"), expected: false},
		{val: []byte("1"), expected: true},
		{val: []byte("true"), expected: true},
		{val: []byte("false"), expected: false},
		{val: []byte("a"), err: "Invalid boolean value a"},
	} {
		got, err := d.boolean(c.val)
		if !errorContains(err, c.err) {
			t.Fatal(err)
		}
		if got != c.expected {
			t.Fatalf("Expected %v, got %v", c.expected, got)
		}
	}
}
