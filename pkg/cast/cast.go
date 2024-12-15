package cast

import (
	"unsafe"
)

func StringToByteArray(str string) []byte {
	return unsafe.Slice(unsafe.StringData(str), len(str))
}

func ByteArrayToString(arr []byte) string {
	return unsafe.String(unsafe.SliceData(arr), len(arr))
}
