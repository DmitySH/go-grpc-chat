package cryptotransfer

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

var DefaultLabel = []byte("DmitySH chat OAEP Encrypted")

func GenerateKeyPair(bits int) (*rsa.PrivateKey, error) {
	keyPair, generateErr := rsa.GenerateKey(rand.Reader, bits)
	if generateErr != nil {
		return nil, fmt.Errorf("can't generate key: %w", generateErr)

	}

	if validateErr := keyPair.Validate(); validateErr != nil {
		return nil, fmt.Errorf("key pair validation failed: %w", validateErr)
	}

	return keyPair, nil
}

func EncodePublicKeyToBase64(pubKey *rsa.PublicKey) string {
	pubKeyBytes := x509.MarshalPKCS1PublicKey(pubKey)

	return base64.StdEncoding.EncodeToString(pubKeyBytes)
}

func DecodePublicKeyFromBase64(encodedKey string) (*rsa.PublicKey, error) {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("can't decode string: %w", err)
	}

	publicKey, err := x509.ParsePKCS1PublicKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("can't parse key: %w", err)
	}

	return publicKey, nil
}

func EncryptRSAMessage(secretMessage string, pubKey *rsa.PublicKey) (string, error) {
	cipherText, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, []byte(secretMessage), DefaultLabel)
	if err != nil {
		return "", fmt.Errorf("can't encrypt OAEP: %w", err)
	}

	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func DecryptRSAMessage(cipherText string, privKey *rsa.PrivateKey) (string, error) {
	encryptedBytes, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("can't decode string: %w", err)
	}

	message, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, encryptedBytes, DefaultLabel)
	if err != nil {
		return "", fmt.Errorf("can't decrypt OAEP: %w", err)
	}

	return string(message), nil
}
