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

func TestReplaceByteArraysWithPlaceholders(t *testing.T) {
	ua := [5]uint{1, 2, 3, 4, 5}
	ia := [5]int{-7, -8, -9, -10, -11}

	t.Run("WithNoByteArrays", func(t *testing.T) {
		test := struct {
			I8   int8
			I16  int16
			I32  int32
			I64  int64
			U8   uint8
			U16  uint16
			U32  uint32
			U64  uint64
			F32  float32
			F64  float64
			FLIA [5]int
			FLUA [5]uint
			YN   bool
		}{
			int8(37),
			int16(291),
			int32(532),
			int64(223),
			uint8(1),
			uint16(55),
			uint32(94),
			uint64(86),
			float32(3.14235523),
			float64(235423523.92035923805),
			ia,
			ua,
			true,
		}

		results, attachments := replaceByteArraysWithPlaceholders(test, make([][]byte, 0))

		if len(attachments) != 0 {
			t.Fatal("Unexpected attachments")
		}

		if r, ok := results.(map[string]interface{}); !ok {
			t.Fatalf("Expected map[string]interface{}, got %v\n", r)
		}

	})
}

func TestHasBinary(t *testing.T) {
	if HasBinary(nil) {
		t.Fatal("nil is not binary data")
	}

	data := []interface{}{int8(math.MaxInt8), int16(math.MaxInt16), int32(math.MaxInt32), int64(math.MaxInt64), uint8(math.MaxUint8), uint16(math.MaxUint16), uint32(math.MaxUint32), uint64(math.MaxUint64), float32(math.MaxFloat32), float64(math.MaxFloat64), "hello", complex64(1i), complex128(222i), false, int(0), uint(2)}

	if HasBinary(data) {
		t.Fatalf("%v does not have binary data\n", data)
	}

	data = []interface{}{[]byte{1, 2, 3}}

	if !HasBinary(data) {
		t.Fatalf("%v has binary data\n", data)
	}

	fixedLength := [5]byte{1, 2, 3, 4, 5}

	if !HasBinary(fixedLength) {
		t.Fatalf("%v has binary data\n", fixedLength)
	}

	structData := simpleHasBinary{Data: []byte{2, 5, 6}}

	if !HasBinary(structData) {
		t.Fatalf("%v has binary data\n", structData)
	}

	nestedData := nestedHasBinary{
		Data: []interface{}{[]byte{15, 20, 25}},
	}

	if !HasBinary(nestedData) {
		t.Fatalf("%v has binary data\n", nestedData)
	}

	noBinaryStruct := simpleNoBinary{}

	if HasBinary(noBinaryStruct) {
		t.Fatalf("%v has no binary data", noBinaryStruct)
	}

}
