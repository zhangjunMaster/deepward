package util

import "bytes"

func Concat(src1 []byte, src2 []byte) []byte {
	var buffer bytes.Buffer
	buffer.Write(src1)
	buffer.Write(src2)
	data := buffer.Bytes()
	return data
}
