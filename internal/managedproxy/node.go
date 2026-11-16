package managedproxy

import (
	"encoding/hex"
	"encoding/pem"
	"errors"
	"net"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Node contains only the settings exposed by the manual node editor. Core
// routing, listeners and outbound transport options are generated separately.
type Node struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Type                 string   `json:"type"`
	Server               string   `json:"server"`
	Port                 int      `json:"port"`
	Username             string   `json:"username,omitempty"`
	AuthMethod           string   `json:"auth_method,omitempty"`
	Password             string   `json:"password,omitempty"`
	PrivateKey           string   `json:"private_key,omitempty"`
	PrivateKeyPassphrase string   `json:"private_key_passphrase,omitempty"`
	HostKey              []string `json:"host_key,omitempty"`
}

func validToken(value string, length int) bool {
	_, err := hex.DecodeString(value)
	return len(value) == length && err == nil && strings.ToLower(value) == value
}

func (n Node) Validate() error {
	if !validToken(n.ID, 24) {
		return errors.New("代理节点 ID 无效")
	}
	if strings.TrimSpace(n.Name) == "" || utf8.RuneCountInString(n.Name) > 100 || strings.ContainsAny(n.Name, "\x00\r\n") {
		return errors.New("节点名称须为 1–100 个字符，不能包含换行")
	}
	if n.Type != "http" && n.Type != "socks5" && n.Type != "ssh" {
		return errors.New("仅支持 HTTP CONNECT、SOCKS5 和 SSH 代理")
	}
	if n.Server == "" || len(n.Server) > 253 || strings.IndexFunc(n.Server, unicode.IsSpace) >= 0 || strings.ContainsAny(n.Server, "\x00/\\@?#[]") || net.ParseIP(n.Server) == nil && strings.Contains(n.Server, ":") {
		return errors.New("请填写代理服务器的域名或 IP，不包含协议、路径或端口")
	}
	if n.Port < 1 || n.Port > 65535 {
		return errors.New("代理服务器端口应为 1–65535")
	}
	if len(n.Username) > 255 || strings.ContainsAny(n.Username, "\x00\r\n") || len(n.Password) > 8192 || len(n.PrivateKey) > 64<<10 || len(n.PrivateKeyPassphrase) > 8192 || len(n.HostKey) > 32 {
		return errors.New("代理认证信息无效或过长")
	}
	if n.Type != "ssh" {
		if n.AuthMethod != "" || n.PrivateKey != "" || n.PrivateKeyPassphrase != "" || len(n.HostKey) != 0 {
			return errors.New("SSH 认证选项只能用于 SSH 节点")
		}
		if n.Username == "" && n.Password != "" {
			return errors.New("使用代理密码时须填写用户名")
		}
		return nil
	}
	if strings.TrimSpace(n.Username) == "" {
		return errors.New("SSH 节点须填写用户名")
	}
	switch n.AuthMethod {
	case "password":
		if n.Password == "" || n.PrivateKey != "" || n.PrivateKeyPassphrase != "" {
			return errors.New("SSH 密码认证须填写密码，并清除私钥选项")
		}
	case "private-key":
		key := strings.TrimSpace(n.PrivateKey)
		block, rest := pem.Decode([]byte(key))
		if !strings.HasPrefix(key, "-----BEGIN ") || block == nil || !strings.HasSuffix(block.Type, "PRIVATE KEY") || len(strings.TrimSpace(string(rest))) != 0 || n.Password != "" {
			return errors.New("SSH 私钥认证须填写完整私钥内容，并清除密码；不能引用本机文件")
		}
	default:
		return errors.New("请选择 SSH 密码或私钥认证")
	}
	for _, key := range n.HostKey {
		if strings.TrimSpace(key) == "" || len(key) > 16<<10 || strings.ContainsAny(key, "\x00\r\n") {
			return errors.New("SSH 服务器公钥须每行一条，且不能超过 16 KiB")
		}
	}
	return nil
}

func (n Node) coreConfig() map[string]any {
	config := map[string]any{"name": n.ID, "type": n.Type, "server": n.Server, "port": n.Port, "udp": false}
	if n.Username != "" {
		config["username"] = n.Username
	}
	if n.Password != "" {
		config["password"] = n.Password
	}
	if n.Type == "ssh" {
		if n.AuthMethod == "private-key" {
			config["private-key"] = n.PrivateKey
			if n.PrivateKeyPassphrase != "" {
				config["private-key-passphrase"] = n.PrivateKeyPassphrase
			}
		}
		if len(n.HostKey) != 0 {
			config["host-key"] = n.HostKey
		}
	}
	return config
}
