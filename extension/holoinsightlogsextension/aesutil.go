package holoinsightlogsextension

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
)

func AesDecrypt(value, secretKey, iv string) (string, error) {
	if value == "" {
		return "", nil
	}

	keyBytes := make([]byte, aes.BlockSize)
	copy(keyBytes, secretKey)
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	valueBytes := []byte(value)
	var encryptedData = make([]byte, len(valueBytes)/2)
	hex.Decode(encryptedData, valueBytes)

	result := make([]byte, len(encryptedData))
	if iv == "" {
		blocksize := block.BlockSize()
		temp := result
		for len(encryptedData) > 0 {
			block.Decrypt(temp, encryptedData[:blocksize])
			encryptedData = encryptedData[blocksize:]
			temp = temp[blocksize:]
		}
	} else {
		ivBytes := make([]byte, aes.BlockSize)
		copy(ivBytes, iv)

		blockMode := cipher.NewCBCDecrypter(block, ivBytes)
		if len(encryptedData)%aes.BlockSize != 0 {
			return value, errors.New("not encrypted apikey")
		}
		blockMode.CryptBlocks(result, encryptedData)
	}

	unpadding := int(result[len(result)-1])
	result = result[:(len(result) - unpadding)]
	return string(result), nil
}
