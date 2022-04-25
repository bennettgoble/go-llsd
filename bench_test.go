package llsd

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

type testElement struct {
	String   string        `llsd:"string"`
	Float64  float64       `llsd:"float_64"`
	Float32  float32       `llsd:"float_32"`
	Date     time.Time     `llsd:"date"`
	ID       *UUID         `llsd:"id"`
	Int      int32         `llsd:"int_32"`
	Bool     bool          `llsd:"bool"`
	Binary   []byte        `llsd:"binary"`
	Children []testElement `llsd:"children"`
}

var bytesLLSD []byte
var resultLLSD testElement

func codeInit() {
	f, err := os.Open("testdata/bench.xml.gz")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		panic(err)
	}
	data, err := io.ReadAll(gz)
	if err != nil {
		panic(err)
	}

	bytesLLSD = data

	if err := UnmarshalXML(bytesLLSD, &resultLLSD); err != nil {
		panic("unmarshal data.xml.gz: " + err.Error())
	}

	if data, err = MarshalXML(&resultLLSD); err != nil {
		panic("marshal data.xml.gz: " + err.Error())
	}

	// We don't compare byte equality as there is no gaurantee of map order

	if len(data) != len(bytesLLSD) {
		panic(fmt.Sprintf("different lengths %d %d", len(data), len(bytesLLSD)))
	}
}

func BenchmarkXMLMarshal(b *testing.B) {
	b.ReportAllocs()
	if bytesLLSD == nil {
		b.StopTimer()
		codeInit()
		b.StartTimer()
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := MarshalXML(&resultLLSD); err != nil {
				b.Fatal("MarshalXML: ", err)
			}
		}
	})
	b.SetBytes(int64(len(bytesLLSD)))
}

func BenchmarkXMLUnmarshal(b *testing.B) {
	b.ReportAllocs()
	if bytesLLSD == nil {
		b.StopTimer()
		codeInit()
		b.StartTimer()
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var r testElement
			if err := UnmarshalXML(bytesLLSD, &r); err != nil {
				b.Fatal("UnmarshalXML:", err)
			}
		}
	})
	b.SetBytes(int64(len(bytesLLSD)))
}

func BenchmarkUnmarshalString(b *testing.B) {
	b.ReportAllocs()
	data := []byte(xml.Header + "<llsd><string>hello, world</string></llsd>")
	b.RunParallel(func(pb *testing.PB) {
		var s string
		for pb.Next() {
			if err := UnmarshalXML(data, &s); err != nil {
				b.Fatal("UnmarshalXML:", err)
			}
		}
	})
}
