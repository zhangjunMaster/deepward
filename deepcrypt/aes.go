package deepcrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"math/rand"
	"time"
)

func padding(src []byte, blocksize int) []byte {
	padnum := blocksize - len(src)%blocksize
	pad := bytes.Repeat([]byte{byte(padnum)}, padnum)
	return append(src, pad...)
}

func unpadding(src []byte) []byte {
	n := len(src)
	unpadnum := int(src[n-1])
	return src[:n-unpadnum]
}

func EncryptAES(src []byte, key []byte) []byte {
	block, _ := aes.NewCipher(key)
	src = padding(src, block.BlockSize())
	blockmode := cipher.NewCBCEncrypter(block, key)
	blockmode.CryptBlocks(src, src)
	return src
}

func DecryptAES(src []byte, key []byte) []byte {
	block, _ := aes.NewCipher(key)
	blockmode := cipher.NewCBCDecrypter(block, key)
	blockmode.CryptBlocks(src, src)
	src = unpadding(src)
	return src
}

func Generate128Key(length int) string {
	str := "0123456789!@#$%^&*()_-+={}[]|:;,.?/abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < length; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

//func main() {
//	x := []byte("test iutirtui datatest 45435 datatest gsfgds datatest 5436546 datatest datatest datatest datatest datatest datatest datatest datatest data")
//	keyString := getRandomString(16)
//	key := []byte(keyString)
//	fmt.Println("[len]:", len(key))
//	x1 := EncryptAES(x, key)
//	x2 := DecryptAES(x1, key)
//	fmt.Println(base64.StdEncoding.EncodeToString(x1))
//	fmt.Print(string(x2))
//}
