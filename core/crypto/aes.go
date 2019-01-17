package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"bytes"
	"crypto/md5"
)

var DecryptKey map[string][]byte

func Init()  {
	if DecryptKey==nil {
		DecryptKey = make(map[string][]byte)
	}
}

func AesEncrypt(origData []byte, key []byte) []byte {
	if key==nil{
		return origData
	}

	// 密码生成16位签名
	var k []byte
	sign := md5.Sum(key)
	k = sign[:len(sign)]

	//fmt.Println(origData)

	// 分组秘钥
	block, _ := aes.NewCipher(k)
	// 获取秘钥块的长度
	blockSize := block.BlockSize()
	// 补全码
	origData = PKCS7Padding(origData, blockSize)
	// 加密模式
	blockMode := cipher.NewCBCEncrypter(block, k[:blockSize])
	// 创建数组
	cryted := make([]byte, len(origData))
	// 加密
	blockMode.CryptBlocks(cryted, origData)

	return cryted
}

func AesDecrypt(crytedByte []byte, key []byte) []byte {
	if key==nil{
		return crytedByte
	}

	// 密码生成16位签名
	var k []byte
	sign := md5.Sum(key)
	k = sign[:len(sign)]

	// 分组秘钥
	block, _ := aes.NewCipher(k)
	// 获取秘钥块的长度
	blockSize := block.BlockSize()
	// 加密模式
	blockMode := cipher.NewCBCDecrypter(block, k[:blockSize])
	// 创建数组
	orig := make([]byte, len(crytedByte))

	if len(crytedByte)%blockSize != 0 {
		return nil
	}

	// 解密
	blockMode.CryptBlocks(orig, crytedByte)
	// 去掉补码
	orig = PKCS7UnPadding(orig)

	return orig
}

//补码
func PKCS7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

//去掉补码
func PKCS7UnPadding(origData []byte) []byte {
	var length, unpadding int
	length = len(origData)
	unpadding = int(origData[length - 1])
	if unpadding < length{
		return origData[:(length - unpadding)]
	} else  {
		return origData
	}
}

// 比较密码是否相等
func IsKeyEqual(a,b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	if (a == nil) != (b == nil) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}