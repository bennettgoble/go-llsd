package llsd

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"io"
	"math"
	"os"
	"testing"
)

var binaryBytes []byte

func binaryInit() {
	if len(binaryBytes) > 0 {
		return
	}
	f, err := os.Open("testdata/basic.bin.gz")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		panic(err)
	}
	binaryBytes, err = io.ReadAll(gz)
	if err != nil {
		panic(err)
	}
}

func TestBinaryScan(t *testing.T) {
	binaryInit()
	id, err := hex.DecodeString("67153d5b3659afb48510adda2c034649")
	if err != nil {
		t.Fatal(err)
	}
	f1 := make([]byte, 8)
	binary.BigEndian.PutUint64(f1, math.Float64bits(0.9878624))

	f2 := make([]byte, 8)
	binary.BigEndian.PutUint64(f2, math.Float64bits(100.1))

	bin, err := hex.DecodeString("42696e6172792064617461")
	if err != nil {
		t.Fatal(err)
	}

	expected := []Token{
		MapStart{},
		Key("region_id"),
		Scalar{Type: UUIDType, Data: id},
		Key("scale"),
		Scalar{Type: String, Data: []byte("one minute")},
		Key("simulator statistics"),
		MapStart{},
		Key("time dilation"),
		Scalar{Type: Real, Data: f1},
		MapEnd{},
		Key("array example"),
		ArrayStart{},
		Scalar{Type: Real, Data: f2},
		Scalar{Type: Real, Data: make([]byte, 8)},
		ArrayEnd{},
		Key("base16"),
		Scalar{Type: Binary, Data: bin},
		MapEnd{},
	}
	scanner := NewBinaryScanner(bytes.NewReader(binaryBytes))
	testScan(t, scanner, expected)
}

func TestBinaryBasicUnmarshal(t *testing.T) {
	var dst struct {
		RegionID UUID   `llsd:"region_id"`
		Scale    string `llsd:"scale"`
	}

	err := UnmarshalBinary(binaryBytes, &dst)
	if err != nil {
		t.Fatal(err)
	}
	if dst.Scale != "one minute" {
		t.Fatalf("Expected dst.scale to equal \"%s\", got \"%s\"", "one minute", dst.Scale)
	}
}
