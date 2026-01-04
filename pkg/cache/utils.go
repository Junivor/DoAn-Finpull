package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

// GenerateKey creates a cache key with prefix and ID.
func GenerateKey(prefix string, id string) string {
	return fmt.Sprintf("%s:%s", prefix, id)
}

// GenerateKeyWithParams creates a cache key with multiple parameters.
func GenerateKeyWithParams(prefix string, params ...interface{}) string {
	key := prefix
	for _, param := range params {
		key = fmt.Sprintf("%s:%v", key, param)
	}
	return key
}

// HashKey generates MD5 hash of a key.
func HashKey(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

// BuildPattern creates a Redis pattern for key matching.
func BuildPattern(prefix string) string {
	return fmt.Sprintf("%s*", prefix)
}
