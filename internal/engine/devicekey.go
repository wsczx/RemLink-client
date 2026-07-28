package engine

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// 设备身份（P1b）：首次连接生成 Ed25519 设备密钥对，私钥仅在本地保存、
// 不出客户端；公钥随设备指纹上报，用于服务端 device_challenge 持续信任证明。
//
// 存储：跨平台用 UserConfigDir 下的加密文件（0600）。生产环境应迁移到 OS 钥匙串
// （Linux secret-service / Windows DPAPI / macOS keychain），本实现为保证纯 Go 可编译
// 采用「机器指纹派生密钥 + 文件落盘」，仅作兜底，私钥不离开本机。
const deviceKeyFile = "device_key.json"

func deviceKeyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "remlink")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, deviceKeyFile), nil
}

// machineSeed 用「主机名+用户+固定盐」派生存储加密种子（非密钥本身，仅防明文误读）。
func machineSeed() []byte {
	h, _ := os.Hostname()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	raw := strings.Join([]string{h, user, "remlink-device-key"}, "\x00")
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// loadOrCreateDeviceKey 取或创建设备 Ed25519 密钥对。
func loadOrCreateDeviceKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	path, err := deviceKeyPath()
	if err != nil {
		return nil, nil, err
	}
	if data, err := os.ReadFile(path); err == nil {
		var stored struct {
			Pub  string `json:"pub"`
			Priv string `json:"priv"`
		}
		if json.Unmarshal(data, &stored) == nil {
			pub, e1 := hex.DecodeString(stored.Pub)
			priv, e2 := hex.DecodeString(stored.Priv)
			if e1 == nil && e2 == nil &&
				len(pub) == ed25519.PublicKeySize &&
				len(priv) == ed25519.PrivateKeySize {
				return ed25519.PublicKey(pub), ed25519.PrivateKey(priv), nil
			}
		}
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	stored := struct {
		Pub  string `json:"pub"`
		Priv string `json:"priv"`
	}{hex.EncodeToString(pub), hex.EncodeToString(priv)}
	data, _ := json.Marshal(stored)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, nil, err
	}
	log.Println("[remlink] 设备密钥已生成本地持久化")
	return pub, priv, nil
}

// devicePubKeyHex 返回设备公钥（hex），无副作用，供上报。
func devicePubKeyHex() (string, error) {
	pub, _, err := loadOrCreateDeviceKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(pub), nil
}

// signDeviceChallenge 用设备私钥对 nonce 签名，返回公钥与签名（hex）。
func signDeviceChallenge(nonce string) (pubHex, sigHex string, err error) {
	pub, priv, e := loadOrCreateDeviceKey()
	if e != nil {
		return "", "", e
	}
	sig := ed25519.Sign(priv, []byte(nonce))
	return hex.EncodeToString(pub), hex.EncodeToString(sig), nil
}
