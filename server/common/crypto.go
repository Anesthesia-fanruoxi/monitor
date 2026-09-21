package common

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"sync"
)

// 全局变量存储加密盐
var encryptionKey []byte
var encryptionKeyMu sync.RWMutex // 使用读写锁，允许并发读取

// SetEncryptionKey 设置加密盐（校验密钥长度）
// 密钥非法时返回错误且不覆盖当前密钥：空密钥会让 AES 初始化失败，
// 表现为"所有上报都 400"，必须让调用方知道，而不是打条警告继续跑
func SetEncryptionKey(key string) error {
	switch len(key) {
	case 16, 24, 32:
	default:
		return fmt.Errorf("AES 密钥长度必须为 16/24/32 字节，当前 %d 字节", len(key))
	}

	encryptionKeyMu.Lock()
	defer encryptionKeyMu.Unlock()
	encryptionKey = []byte(key)
	return nil
}

// GetEncryptionKey 获取当前加密盐（线程安全，返回副本防止外部修改全局密钥）
func GetEncryptionKey() []byte {
	encryptionKeyMu.RLock()
	defer encryptionKeyMu.RUnlock()
	if encryptionKey == nil {
		return nil
	}
	out := make([]byte, len(encryptionKey))
	copy(out, encryptionKey)
	return out
}

// Decrypt 解密数据
func Decrypt(ciphertext []byte) ([]byte, error) {
	key := GetEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("密文过短")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
