package handoff

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/scrypt"
)

func loadPrivateKey(path string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, nil, errors.New("invalid PEM private key")
	}
	pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse private key: %w", err)
	}
	key, ok := pk.(ed25519.PrivateKey)
	if !ok {
		return nil, nil, errors.New("private key is not ed25519")
	}
	pub, ok := key.Public().(ed25519.PublicKey)
	if !ok {
		return nil, nil, errors.New("ed25519 public key assertion failed")
	}
	return key, pub, nil
}

func signChecksum(privateKey ed25519.PrivateKey, checksum string) string {
	sig := ed25519.Sign(privateKey, []byte(checksum))
	return base64.StdEncoding.EncodeToString(sig)
}

func verifyBundleSignature(bundle ExportBundle) error {
	if strings.TrimSpace(bundle.Signature) == "" {
		return errors.New("bundle missing signature")
	}
	if bundle.Signer == nil || strings.TrimSpace(bundle.Signer.PublicKey) == "" {
		return errors.New("bundle missing signer public key metadata")
	}
	pubRaw, err := base64.StdEncoding.DecodeString(bundle.Signer.PublicKey)
	if err != nil {
		return fmt.Errorf("decode signer public key: %w", err)
	}
	if len(pubRaw) != ed25519.PublicKeySize {
		return errors.New("invalid signer public key length")
	}
	sig, err := base64.StdEncoding.DecodeString(bundle.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(pubRaw), []byte(bundle.Checksum), sig) {
		return errors.New("bundle signature verification failed")
	}
	return nil
}

func publicKeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:8])
}

func encryptBundle(data []byte, passphrase string) (EncryptedBundle, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return EncryptedBundle{}, fmt.Errorf("salt: %w", err)
	}
	key, err := scrypt.Key([]byte(passphrase), salt, 1<<15, 8, 1, 32)
	if err != nil {
		return EncryptedBundle{}, fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return EncryptedBundle{}, fmt.Errorf("cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedBundle{}, fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return EncryptedBundle{}, fmt.Errorf("nonce: %w", err)
	}
	ct := aead.Seal(nil, nonce, data, nil)
	return EncryptedBundle{
		Version:    1,
		Algorithm:  "aes-256-gcm",
		KDF:        "scrypt(N=32768,r=8,p=1)",
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ct),
	}, nil
}

func decryptBundle(enc EncryptedBundle, passphrase string) ([]byte, error) {
	salt, err := base64.StdEncoding.DecodeString(enc.Salt)
	if err != nil {
		return nil, fmt.Errorf("decode salt: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(enc.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}
	ct, err := base64.StdEncoding.DecodeString(enc.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	key, err := scrypt.Key([]byte(passphrase), salt, 1<<15, 8, 1, 32)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	pt, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, errors.New("decrypt failed: wrong passphrase or corrupted bundle")
	}
	return pt, nil
}
