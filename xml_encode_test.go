package llsd

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestXMLMarshal(t *testing.T) {
	for _, c := range []struct {
		v        any
		expected string
	}{
		{
			v:        struct{ A string }{A: "a"},
			expected: "<map><key>A</key><string>a</string></map>",
		},
		{
			v:        []string{"a", "b"},
			expected: "<array><string>a</string><string>b</string></array>",
		},
		{
			v:        []any{"a", 1, 1.0},
			expected: "<array><string>a</string><integer>1</integer><real>1.000000</real></array>",
		},
		{
			v:        struct{ A []byte }{A: []byte("Binary data")},
			expected: "<map><key>A</key><binary>42696E6172792064617461</binary></map>",
		},
		{
			v:        struct{ A URL }{A: "https://example.org/"},
			expected: "<map><key>A</key><uri>https://example.org/</uri></map>",
		},
	} {
		b, err := MarshalXML(&c.v)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "<llsd>"+c.expected+"</llsd>") {
			got := strings.Replace(string(b), xml.Header, "", 1)
			t.Fatalf("Expected %s, got %s", "<llsd>"+c.expected+"</llsd>", got)
		}
	}
}

func TestXMLOmitUnexportedFromMap(t *testing.T) {
	src := struct{ str string }{str: "a"}
	b, err := MarshalXML(&src)
	if err != nil {
		t.Fatal(err)
	}
	expected := "<llsd><map></map></llsd>"
	if !strings.Contains(string(b), expected) {
		t.Fatalf("Expected %s, got %s", expected, string(b))
	}
}

func TestXMLOmitEmpty(t *testing.T) {
	src := struct {
		A string  `llsd:",omitempty"`
		B int     `llsd:",omitempty"`
		C *string `llsd:",omitempty"`
	}{}
	b, err := MarshalXML(&src)
	if err != nil {
		t.Fatal(err)
	}
	expected := "<llsd><map></map></llsd>"
	if !strings.Contains(string(b), expected) {
		t.Fatalf("Expected %s, got %s", expected, string(b))
	}
}

func TestXMLOmit(t *testing.T) {
	src := struct {
		A string `llsd:"-"`
	}{A: "str"}
	b, err := MarshalXML(&src)
	if err != nil {
		t.Fatal(err)
	}
	expected := "<llsd><map></map></llsd>"
	if !strings.Contains(string(b), expected) {
		t.Fatalf("Expected %s, got %s", expected, string(b))
	}
}

func TestXMLHyphenName(t *testing.T) {
	src := struct {
		A string `llsd:"-,"`
	}{A: "str"}
	b, err := MarshalXML(&src)
	if err != nil {
		t.Fatal(err)
	}
	expected := "<llsd><map><key>-</key><string>str</string></map></llsd>"
	if !strings.Contains(string(b), expected) {
		t.Fatalf("Expected %s, got %s", expected, string(b))
	}
}

func TestXMLEncoding(t *testing.T) {
	b16 := struct {
		A []byte `llsd:",base16"`
	}{[]byte("Binary data")}
	b, err := MarshalXML(&b16)
	if err != nil {
		t.Fatal(err)
	}
	expected := "<llsd><map><key>A</key><binary>42696E6172792064617461</binary></map></llsd>"
	if !strings.Contains(string(b), expected) {
		t.Fatalf("Expected %s, got %s", expected, string(b))
	}

	b16Implicit := struct {
		A []byte
	}{[]byte("Binary data")}
	b, err = MarshalXML(&b16Implicit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), expected) {
		t.Fatalf("Expected %s, got %s", expected, string(b))
	}

	b64 := struct {
		A []byte `llsd:",base64"`
	}{[]byte("Binary data")}
	b, err = MarshalXML(&b64)
	if err != nil {
		t.Fatal(err)
	}
	expected = `<llsd><map><key>A</key><binary encoding="base64">QmluYXJ5IGRhdGE=</binary></map></llsd>`
	if !strings.Contains(string(b), expected) {
		t.Fatalf("Expected %s, got %s", expected, string(b))
	}

	b85 := struct {
		A []byte `llsd:",base85"`
	}{[]byte("Binary data")}
	b, err = MarshalXML(&b85)
	if err != nil {
		t.Fatal(err)
	}
	expected = `<llsd><map><key>A</key><binary encoding="base85">6&gt;:=GEd8d&lt;@&lt;&gt;oX</binary></map></llsd>`
	if !strings.Contains(string(b), expected) {
		t.Fatalf("Expected %s, got %s", expected, string(b))
	}
}
