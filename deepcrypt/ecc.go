package deepcrypt

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
)

func getKey() (*ecdsa.PrivateKey, error) {
	prk, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return prk, err
	}
	return prk, nil
}

func EccSign(data []byte, prk *ecdsa.PrivateKey) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, prk, data)
	if err != nil {
		return nil, err
	}
	params := prk.Curve.Params()
	curveOrderByteSize := params.P.BitLen() / 8
	rBytes, sBytes := r.Bytes(), s.Bytes()
	signature := make([]byte, curveOrderByteSize*2)
	copy(signature[curveOrderByteSize-len(rBytes):], rBytes)
	copy(signature[curveOrderByteSize*2-len(sBytes):], sBytes)
	return signature, nil
}

func EccVerify(data, signature []byte, puk *ecdsa.PublicKey) bool {
	curveOrderByteSize := puk.Curve.Params().P.BitLen() / 8
	r, s := new(big.Int), new(big.Int)
	r.SetBytes(signature[:curveOrderByteSize])
	s.SetBytes(signature[curveOrderByteSize:])
	return ecdsa.Verify(puk, data, r, s)
}

func ECCEncrypt(pt []byte, puk *ecdsa.PublicKey) ([]byte, error) {
	epuk := ecies.ImportECDSAPublic(puk)
	ct, err := ecies.Encrypt(rand.Reader, epuk, pt, nil, nil)
	return ct, err
}

func ECCDecrypt(ct []byte, prk *ecdsa.PrivateKey) ([]byte, error) {
	eprk := ecies.ImportECDSA(prk)
	pt, err := eprk.Decrypt(ct, nil, nil)
	return pt, err
}

func ByteToBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func Base64ToByte(b string) ([]byte, error) {
	decodeBytes, err := base64.StdEncoding.DecodeString(b)
	return decodeBytes, err
}

func ExportKey() (string, string) {
	prk, err := getKey()
	puk := &prk.PublicKey
	if err != nil {
		panic(err)
	}
	bprk := crypto.FromECDSA(prk)
	bpuk := crypto.FromECDSAPub(puk)

	//3.对ecdsa的公私钥进行base64,生成可存储的公私钥对
	sprk := ByteToBase64(bprk)
	spuk := ByteToBase64(bpuk)
	return sprk, spuk
}

func Encrypt(ct []byte, spuk string) ([]byte, error) {
	bpuk, err := Base64ToByte(spuk)
	if err != nil {
		return nil, err
	}
	puk, err := crypto.UnmarshalPubkey(bpuk)
	if err != nil {
		return nil, err
	}
	endata, err := ECCEncrypt(ct, puk)
	if err != nil {
		return nil, err
	}
	return endata, nil
}

func Decrypt(ct []byte, sprk string) ([]byte, error) {
	bprk, err := Base64ToByte(sprk)
	if err != nil {
		return nil, err
	}
	prk, err := crypto.ToECDSA(bprk)
	if err != nil {
		return nil, err
	}
	data, err := ECCDecrypt(ct, prk)
	if err != nil {
		return nil, err
	}
	fmt.Println("[decrypto]:", string(data), err)
	return data, nil
}

func main() {
	data := "11234567890qazwsxedcrfvtgbyhnujmik,ol.p;/123456789qwertyuiopasdfghjkl"
	// 1.生成 ecdsa的私钥和公钥
	prk, err := getKey()
	puk := &prk.PublicKey
	if err != nil {
		panic(err)
	}
	//2.生成ecdsa私钥和公钥的 []byte
	bprk := crypto.FromECDSA(prk)
	bpuk := crypto.FromECDSAPub(puk)

	//3.对ecdsa的公私钥进行base64,生成可存储的公私钥对
	sprk := ByteToBase64(bprk)
	spuk := ByteToBase64(bpuk)
	fmt.Printf("spuk: %s\nsprk: %s \n", spuk, sprk)

	// 4.将公私钥对转为 [] byte
	bprk, err = Base64ToByte(sprk)
	if err != nil {
		fmt.Println("[Base64ToByte err]:", err)
	}
	bpuk, err = Base64ToByte(spuk)
	if err != nil {
		fmt.Println("[Base64ToByte err]:", err)
	}
	//5.[]byte => key
	prk, err = crypto.ToECDSA(bprk)
	puk, err = crypto.UnmarshalPubkey(bpuk)
	if err != nil {
		fmt.Println("[UnmarshalPubkey err]:", err)
	}
	// 6.加密
	endata, err := ECCEncrypt([]byte(data), puk)
	fmt.Println("[endata, err]", err)

	// 7.解密
	ddata, err := ECCDecrypt(endata, prk)
	fmt.Println("[decrypto]:", string(ddata), err)

	//测试是否转换成功,prk是ecdsa pri, lpuk是ecdsa puk
	eccData, err := EccSign([]byte(data), prk)
	if err != nil {
		panic(err)
	}
	fmt.Println(EccVerify([]byte(data), eccData, puk))
	fmt.Println(data)
}
