package util

import "bytes"

var lable2byte = map[string][]byte{
	"ecc": []byte{0, 0},
	"aes": []byte{0, 1},
}

func Concat(src1 []byte, src2 []byte) []byte {
	var buffer bytes.Buffer
	buffer.Write(src1)
	buffer.Write(src2)
	data := buffer.Bytes()
	return data
}

func GenMsg(lable string, data []byte) []byte {
	lableByte := lable2byte[lable]
	return Concat(lableByte, data)
}
