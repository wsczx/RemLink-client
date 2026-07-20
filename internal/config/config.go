// Package config 管理客户端本地配置（已保存的服务器、上次选择等）。
//
// 凭据默认以明文存于用户配置目录（受 OS 文件权限保护）。
// 如需加密落盘，可后续复用服务端 pkg/security 的 AES-GCM 封装。
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"remlink-client/internal/types"
)

// AppConfig 是客户端持久化配置。
type AppConfig struct {
	Servers      []types.ServerEntry `json:"servers"`
	LastServer   string              `json:"last_server"`
	AutoReconnect bool               `json:"auto_reconnect"`
}

// Path 返回配置文件路径（跨平台用户配置目录）。
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "remlink")
	if err := os.MkdirAll(p, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(p, "config.json"), nil
}

// Load 读取配置，文件不存在时返回空配置。
func Load() (*AppConfig, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &AppConfig{}, nil
		}
		return nil, err
	}
	var c AppConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save 写回配置。
func (c *AppConfig) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
