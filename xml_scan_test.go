package llsd

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

const xmlStr = `<?xml version="1.0" encoding="UTF-8"?>
<llsd>
<map>
	<key>region_id</key><uuid>67153d5b-3659-afb4-8510-adda2c034649</uuid>
	<key>scale</key><string>one minute</string>
	<key>simulator statistics</key>
	<map>
	  <key>time dilation</key><real>0.9878624</real>
	</map>
	<key>array example</key>
	<array>
	  <real>100.1</real>
	  <real />
	</array>
	<key>binary examples</key>
	<map>
	  <key>empty binary</key><binary />
	  <key>base16</key><binary encoding="base16">42696e6172792064617461</binary>
	  <key>base64</key><binary encoding="base64">QmluYXJ5IGRhdGE=</binary>
	  <key>base85</key><binary encoding="base85">6&gt;:=GEd8d&lt;@&lt;&gt;o</binary>
	</map>
</map>
</llsd>`

// testScan will compare a list of expected tokens to those it reads from a
// TokenReader.
func testScan(t *testing.T, scanner TokenReader, expected []Token) {
	for i, el := range expected {
		got, err := scanner.Token()
		if err != nil {
			t.Fatal(err)
		}
		gotType := reflect.TypeOf(got)
		elType := reflect.TypeOf(el)
		if gotType != elType {
			t.Fatalf("Expected element %d to be type %s, got %s", i, reflect.TypeOf(el), reflect.TypeOf(got))
		}
		switch el.(type) {
		case Key:
			if el != got {
				t.Fatalf("Expected key %s=%s", el, got)
			}
		case Scalar:
			expectedScalar := el.(Scalar)
			gotScalar := got.(Scalar)
			if expectedScalar.Type != gotScalar.Type {
				t.Fatalf("Expected element %d to have scalar type %s but got %s", i, expectedScalar.Type, gotScalar.Type)
			}
			if bytes.Compare(expectedScalar.Data, gotScalar.Data) != 0 {
				t.Fatalf("Expected element %d to have value \"%s\" but got \"%s\"", i, expectedScalar.Data, gotScalar.Data)
			}
			if len(expectedScalar.Attr) > 0 {
				for k, v := range expectedScalar.Attr {
					gotVal, ok := gotScalar.Attr[k]
					if !ok {
						t.Fatalf("Expected element %d to have attribute %s", i, k)
					}
					if v != gotVal {
						t.Fatalf("Expected element %d attribute %s to equal %s but got %s", i, k, v, gotVal)
					}
				}
			}
		}
	}
	_, err := scanner.Token()
	if err != io.EOF {
		t.Fatalf("Expected EOF")
	}
}

func TestXMLScan(t *testing.T) {
	expected := []Token{
		MapStart{},
		Key("region_id"),
		Scalar{Type: UUIDType, Data: []byte("67153d5b-3659-afb4-8510-adda2c034649")},
		Key("scale"),
		Scalar{Type: String, Data: []byte("one minute")},
		Key("simulator statistics"),
		MapStart{},
		Key("time dilation"),
		Scalar{Type: Real, Data: []byte("0.9878624")},
		MapEnd{},
		Key("array example"),
		ArrayStart{},
		Scalar{Type: Real, Data: []byte("100.1")},
		Scalar{Type: Real},
		ArrayEnd{},
		Key("binary examples"),
		MapStart{},
		Key("empty binary"),
		Scalar{Type: Binary},
		Key("base16"),
		Scalar{Type: Binary, Data: []byte("42696e6172792064617461"), Attr: map[string]string{"encoding": "base16"}},
		Key("base64"),
		Scalar{Type: Binary, Data: []byte("QmluYXJ5IGRhdGE="), Attr: map[string]string{"encoding": "base64"}},
		Key("base85"),
		Scalar{Type: Binary, Data: []byte("6>:=GEd8d<@<>o"), Attr: map[string]string{"encoding": "base85"}},
		MapEnd{},
		MapEnd{},
	}
	scanner := NewXMLScanner(strings.NewReader(xmlStr))
	testScan(t, scanner, expected)
}

func TestXMLUnmarshalScalar(t *testing.T) {
	for _, c := range []struct {
		element   string
		innerText string
		expected  any
	}{
		{"real", "1.2", float64(1.2)},
		{"integer", "1", 1},
		{"string", "v", "v"},
	} {
		dst := reflect.New(reflect.TypeOf(c.expected))
		xml := `<?xml version="1.0" encoding="UTF-8"?><llsd><` + c.element + `>` + c.innerText + `</` + c.element + `></llsd>`
		err := NewXMLDecoder(strings.NewReader(xml)).Unmarshal(dst.Interface())
		if err != nil {
			t.Error(err)
		}
		if dst.Elem().Interface() != c.expected {
			t.Errorf("Expected unmarshaled %s to equal \"%v\" but got \"%v\"", c.element, c.expected, dst.Elem())
		}
	}
}

func TestXMLUnmarshalTypeError(t *testing.T) {
	type T struct{}

	for _, c := range []struct {
		element   string
		innerText string
	}{
		{"real", "1.2"},
		{"integer", "1"},
		{"string", "v"},
	} {
		var dst T
		xml := `<?xml version="1.0" encoding="UTF-8"?><llsd><` + c.element + `>` + c.innerText + `</` + c.element + `></llsd>`
		err := NewXMLDecoder(strings.NewReader(xml)).Unmarshal(&dst)
		expected := "LLSD: Cannot unmarshal " + c.element + " " + c.innerText + " into Go value of type llsd.T."
		if err.Error() != expected {
			t.Fatalf("Expected error \"%s\" but got \"%s\"", expected, err)
		}
		elementStart := "<" + c.element + ">"
		// Current offset returned by the XML parser is that of the EndElement
		// This could possibly be updated to return the StartElement position
		expectedOffset := strings.Index(xml, elementStart) + len(elementStart) + len(c.innerText) + len(elementStart) + 1
		if err.(*UnmarshalTypeError).Offset != int64(expectedOffset) {
			t.Fatalf("Expected UnmarshalTypeError (%s) to report offset %d but got %d", c.element, expectedOffset, err.(*UnmarshalTypeError).Offset)
		}
	}
}

type csv []string

func (c *csv) UnmarshalTextLLSD(b []byte) error {
	v := strings.Split(string(b), ",")
	*c = append(*c, v...)
	return nil
}

func TestXMLCustomScalarUnmarshal(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?><llsd><string>a,b</string></llsd>`
	var dst csv
	err := UnmarshalXML([]byte(xml), &dst)
	if err != nil {
		t.Fatal(err)
	}
	if dst[0] != "a" || dst[1] != "b" {
		t.Fatalf("Expected custom TextUnmarshaler to correctly deserialize CSV values")
	}
}

func TestXMLDisallowUnknownFields(t *testing.T) {
	var dst struct{}
	xml := `<?xml version="1.0" encoding="UTF-8"?><llsd><map><key>a</key><string>a</string></llsd>`
	dec := NewXMLDecoder(strings.NewReader(xml))
	dec.DisallowUnknownFields = true
	err := dec.Unmarshal(&dst)
	expectedErr := "LLSD: Unknown field \"a\""
	if err.Error() != expectedErr {
		t.Fatalf("Expected error to equal \"%s\" but got \"%s\"", expectedErr, err.Error())
	}
}

func TestXMLBasicUnmarshal(t *testing.T) {
	var dst struct {
		String    string
		Real      float64
		Boolean   bool
		URI       URL
		Binary    []byte
		BinaryArr [11]byte
		Date      time.Time
		Object    struct {
			B string
			C string `llsd:"c"`
			D *string
			E *string
			F *string
		}
	}
	xml := `<?xml version="1.0" encoding="UTF-8"?>
	<llsd>
	  <map>
	  	<key>String</key><string>a</string>
		<key>Real</key><real>1.0</real>
		<key>Boolean</key><boolean>true</boolean>
		<key>URI</key><uri>http://example.org</uri>
		<key>Binary</key><binary>42696e6172792064617461</binary>
		<key>BinaryArr</key><binary>42696e6172792064617461</binary>
		<key>Undef</key><undef/><key>StringAfterUndef</key><string>A</string>
		<key>Object</key>
		<map>
		  <key>B</key><string>b</string>
		  <key>c</key><string>c</string>
		  <key>d</key><string>d</string>
		  <key>E</key><string>e</string>
		  <key>F</key><undef />
		</map>
	  </map>
	</llsd>`
	err := UnmarshalXML([]byte(xml), &dst)
	if err != nil {
		t.Fatal(err)
	}
	if dst.String != "a" {
		t.Fatalf("Expected dst.String to equal \"a\" but got \"%s\"", dst.String)
	}
	if dst.Real != 1.0 {
		t.Fatalf("Expected dst.Real to equal \"1.0\" but got \"%f\"", dst.Real)
	}
	if dst.Boolean != true {
		t.Fatalf("Expected dst.Boolean to equal \"true\" but got \"%v\"", dst.Boolean)
	}
	if bytes.Compare(dst.Binary, []byte("Binary data")) != 0 {
		t.Fatalf("Expected dst.Binary to equal \"Binary data\" but got \"%s\"", dst.Binary)
	}
	if bytes.Compare(dst.BinaryArr[:], []byte("Binary data")) != 0 {
		t.Fatalf("Expected dst.Binary to equal \"Binary data\" but got \"%s\"", dst.BinaryArr)
	}
	if dst.URI != "http://example.org" {
		t.Fatalf("Expected dst.URI to equal \"http://example.org\" but got \"%s\"", dst.URI)
	}
	if dst.Object.B != "b" {
		t.Fatalf("Expected dst.Object.B to equal \"b\" but got \"%s\"", dst.Object.B)
	}
	if dst.Object.C != "c" {
		t.Fatalf("Expected dst.Object.C to equal \"c\" but got \"%s\"", dst.Object.C)
	}
	if dst.Object.D != nil {
		t.Fatalf("Expected dst.Object.D to equal \"nil\" but got \"%s\"", *dst.Object.D)
	}
	if *dst.Object.E != "e" {
		t.Fatalf("Expected dst.Object.E to equal \"nil\" but got \"%s\"", *dst.Object.E)
	}
	if dst.Object.F != nil {
		t.Fatalf("Expected dst.Object.F to equal \"nil\" but got \"%s\"", *dst.Object.F)
	}
}

// Test unmarshaling and conversion rules for date LLSD values
func TestXMLParseDate(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	nowTs := now.Format(time.RFC3339)

	epoch := time.Unix(0, 0)
	epochTs := epoch.Format(time.RFC3339)

	var dst struct {
		Time    time.Time
		TimePtr *time.Time
		Integer int
		Real    float64
		Epoch   time.Time
	}
	xml := `<?xml version="1.0" encoding="UTF-8"?>
	<llsd>
	  <map>
	  	<key>Time</key><date>` + nowTs + `</date>
	  	<key>TimePtr</key><date>` + nowTs + `</date>
	  	<key>Integer</key><date>` + nowTs + `</date>
	  	<key>Real</key><date>` + nowTs + `</date>
	  	<key>Epoch</key><date />
	  </map>
	</llsd>`
	err := UnmarshalXML([]byte(xml), &dst)
	if err != nil {
		t.Fatal(err)
	}
	if !dst.Time.Equal(now) {
		t.Fatalf("Expected dst.Time to equal \"%s\" got \"%s\"", nowTs, dst.Time.Format(time.RFC3339))
	}
	if !dst.TimePtr.Equal(now) {
		t.Fatalf("Expected dst.TimePtr to equal \"%s\" got \"%s\"", nowTs, dst.TimePtr.Format(time.RFC3339))
	}
	if int64(dst.Integer) != now.Unix() {
		t.Fatalf("Expected dst.Integer to equal \"%d\" got \"%d\"", now.Unix(), dst.Integer)
	}
	if dst.Real != float64(now.Unix()) {
		t.Fatalf("Expected dst.Real to equal \"%f\" got \"%f\"", float64(now.Unix()), dst.Real)
	}
	if !dst.Epoch.Equal(epoch) {
		t.Fatalf("Expected dst.Epoch to equal \"%s\" got \"%s\"", epochTs, dst.Epoch.Format(time.RFC3339))
	}
}

// Test unmarshaling and conversion rules for binary LLSD values
func TestXMLParseBinary(t *testing.T) {
	var dst struct {
		Slice   []byte
		Array   [11]byte
		String  string
		Int32   int32
		Int64   int64
		Boolean bool
	}
	xml := `<?xml version="1.0" encoding="UTF-8"?>
	<llsd>
	  <map>
	  	<key>Slice</key><binary>42696e6172792064617461</binary>
	  	<key>Array</key><binary>42696e6172792064617461</binary>
	  	<key>String</key><binary>42696e6172792064617461</binary>
	  	<key>Int32</key><binary>FFFFFFFD</binary>
	  	<key>Int64</key><binary>FFFFFFFFFFFFFFFD</binary>
	  	<key>Boolean</key><binary>FF</binary>
	  </map>
	</llsd>`
	err := UnmarshalXML([]byte(xml), &dst)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(dst.Slice, []byte("Binary data")) != 0 {
		t.Fatalf("Expected dst.Slice to equal \"Binary data\" got \"%s\"", string(dst.Slice))
	}
	if dst.Int32 != -3 {
		t.Fatalf("Expected dst.Int32 to equal \"-3\" but got \"%d\"", dst.Int32)
	}
	if dst.Int64 != -3 {
		t.Fatalf("Expected dst.Int64 to equal \"-3\" but got \"%d\"", dst.Int64)
	}
	if !dst.Boolean {
		t.Fatalf("Expected dst.Boolean to equal \"true\" but got \"%v\"", dst.Boolean)
	}
}

func TestXMLUnmarshalPointer(t *testing.T) {
	var dst1 struct{ A *string }
	xml := `<?xml version="1.0" encoding="UTF-8"?>
	<llsd>
	  <map>
	  	<key>A</key><string>a</string>
	  </map>
	</llsd>`
	err := UnmarshalXML([]byte(xml), &dst1)
	if err != nil {
		t.Fatal(err)
	}
	if *dst1.A != "a" {
		t.Fatalf("Expected dst.A to equal \"a\" but got \"%s\"", *dst1.A)
	}

	var dst2 struct{ A *struct{ B string } }
	xml = `<?xml version="1.0" encoding="UTF-8"?>
	<llsd>
	  <map>
	  	<key>A</key>
		<map>
		  <key>B</key>
		  <string>b</string>
		</map>
	  </map>
	</llsd>`
	err = UnmarshalXML([]byte(xml), &dst2)
	if err != nil {
		t.Fatal(err)
	}
	if dst2.A.B != "b" {
		t.Fatalf("Expected dst.A to equal \"a\" but got \"%s\"", dst2.A.B)
	}
}

func TestXMLUnmarhsalUsingJSONTags(t *testing.T) {
	var dst struct {
		A string `json:"a"`
		B string `json:"b,omitempty"`
		C string `json:",omitempty"`
	}
	xml := `<?xml version="1.0" encoding="UTF-8"?>
	<llsd>
	  <map>
	  	<key>a</key><string>a</string>
	  	<key>b</key><string>b</string>
	  	<key>C</key><string>c</string>
	  </map>
	</llsd>`
	err := UnmarshalXML([]byte(xml), &dst)
	if err != nil {
		t.Fatal(err)
	}
	if dst.A != "a" {
		t.Fatalf("Expected dst.A to equal \"a\" but got \"%s\"", dst.A)
	}
	if dst.B != "b" {
		t.Fatalf("Expected dst.B to equal \"b\" but got \"%s\"", dst.B)
	}
	if dst.C != "c" {
		t.Fatalf("Expected dst.C to equal \"c\" but got \"%s\"", dst.C)
	}
}

func TestXMLUnmarshalAny(t *testing.T) {
	var dst struct {
		Binary  any
		Date    any
		Integer any
		Real    any
		String  any
		URI     any
	}
	xml := `<?xml version="1.0" encoding="UTF-8"?>
	<llsd>
	  <map>
		<key>Binary</key><binary>42696e6172792064617461</binary>
		<key>Date</key><date>2006-02-01T14:29:53.43Z</date>
		<key>Integer</key><integer>1</integer>
	  	<key>Real</key><real>1.0</real>
		<key>String</key><string>str</string>
		<key>URI</key><uri>http://example.org</uri>
	  </map>
	</llsd>`
	err := UnmarshalXML([]byte(xml), &dst)
	if err != nil {
		t.Fatal(err)
	}
	if dst.Real.(float64) != 1.0 {
		t.Fatalf("Expected dst.Real to equal \"1.0\" but got \"%f\"", dst.Real)
	}
}

// func BenchmarkXMLUnmarshal(b *testing.B) {
// 	b.ReportAllocs()

// }

func TestXMLUnmarshalMap(t *testing.T) {
	dst := map[string]string{}
	xml := `<?xml version="1.0" encoding="UTF-8"?>
	<llsd>
	  <map>
	  	<key>a</key><string>a</string>
	  	<key>b</key><string>b</string>
	  </map>
	</llsd>`
	err := UnmarshalXML([]byte(xml), &dst)
	if err != nil {
		t.Fatal(err)
	}
	if dst["a"] != "a" {
		t.Fatalf("Expected dst[a] to equal \"a\" but got %s", dst["a"])
	}
	if dst["b"] != "b" {
		t.Fatalf("Expected dst[b] to equal \"b\" but got %s", dst["b"])
	}

	dst2 := map[string]*string{}
	err = UnmarshalXML([]byte(xml), &dst2)
	if err != nil {
		t.Fatal(err)
	}
	if *dst2["a"] != "a" {
		t.Fatalf("Expected dst[a] to equal \"a\" but got %s", *dst2["a"])
	}
	if *dst2["b"] != "b" {
		t.Fatalf("Expected dst[b] to equal \"b\" but got %s", *dst2["b"])
	}

	dst3 := map[string]any{}
	xml = `<?xml version="1.0" encoding="UTF-8"?>
	<llsd>
	  <map>
	  	<key>a</key><string>a</string>
		<key>b</key><binary>42696e6172792064617461</binary>
	  </map>
	</llsd>`
	err = UnmarshalXML([]byte(xml), &dst3)
	if err != nil {
		t.Fatal(err)
	}
	if dst3["a"] != "a" {
		t.Fatalf("Expected dst3[a] to equal \"a\" but got %s", dst3["a"])
	}
	if bytes.Compare(dst3["b"].([]byte), []byte("Binary data")) != 0 {
		t.Fatalf("Expected dst3[b] to equal \"b\" but got %s", dst3["b"])
	}
}
