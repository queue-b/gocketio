package socket

import (
	"math"
	"testing"
)

type simpleHasBinary struct {
	Data []byte
}

type nestedHasBinary struct {
	Something int
	Data      []interface{}
}

type simpleNoBinary struct {
	Data float32
}

func TestHasBinary(t *testing.T) {
	if hasBinary(nil) {
		t.Fatal("nil is not binary data")
	}

	data := []interface{}{int8(math.MaxInt8), int16(math.MaxInt16), int32(math.MaxInt32), int64(math.MaxInt64), uint8(math.MaxUint8), uint16(math.MaxUint16), uint32(math.MaxUint32), uint64(math.MaxUint64), float32(math.MaxFloat32), float64(math.MaxFloat64), "hello", complex64(1i), complex128(222i), false, int(0), uint(2)}

	if hasBinary(data) {
		t.Fatalf("%v does not have binary data\n", data)
	}

	data = []interface{}{[]byte{1, 2, 3}}

	if !hasBinary(data) {
		t.Fatalf("%v has binary data\n", data)
	}

	fixedLength := [5]byte{1, 2, 3, 4, 5}

	if !hasBinary(fixedLength) {
		t.Fatalf("%v has binary data\n", fixedLength)
	}

	structData := simpleHasBinary{Data: []byte{2, 5, 6}}

	if !hasBinary(structData) {
		t.Fatalf("%v has binary data\n", structData)
	}

	nestedData := nestedHasBinary{
		Data: []interface{}{[]byte{15, 20, 25}},
	}

	if !hasBinary(nestedData) {
		t.Fatalf("%v has binary data\n", nestedData)
	}

	noBinaryStruct := simpleNoBinary{}

	if hasBinary(noBinaryStruct) {
		t.Fatalf("%v has no binary data", noBinaryStruct)
	}

}
