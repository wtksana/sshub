package library

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"
)

type Profile struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	Username     string    `json:"username"`
	Auth         string    `json:"auth"`
	IdentityPath string    `json:"identity_path,omitempty"`
	Group        string    `json:"group,omitempty"`
	Favorite     bool      `json:"favorite"`
	LastUsed     time.Time `json:"last_used,omitempty"`
}

func Validate(p Profile) error {
	for _, field := range []struct{ name, value string }{{"名称", p.Name}, {"地址", p.Host}, {"用户名", p.Username}, {"分组", p.Group}, {"私钥路径", p.IdentityPath}} {
		if len(field.value) > 4096 || strings.IndexFunc(field.value, unicode.IsControl) >= 0 {
			return fmt.Errorf("%s过长或包含控制字符", field.name)
		}
	}
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Host) == "" || strings.TrimSpace(p.Username) == "" {
		return errors.New("请填写名称、地址和用户名")
	}
	if strings.ContainsAny(p.Host, "/\\@[]") || strings.IndexFunc(p.Host, unicode.IsSpace) >= 0 || (strings.Contains(p.Host, ":") && net.ParseIP(p.Host) == nil) {
		return errors.New("地址只填写主机名或 IP，端口请填写在端口栏；IPv6 不加方括号")
	}
	if strings.IndexFunc(p.Username, unicode.IsSpace) >= 0 || strings.Contains(p.Username, "@") {
		return errors.New("用户名不能包含空白或 @")
	}
	if p.Port < 1 || p.Port > 65535 {
		return errors.New("端口必须为 1–65535")
	}
	switch p.Auth {
	case "password", "agent":
	case "private_key":
		if strings.TrimSpace(p.IdentityPath) == "" {
			return errors.New("请选择私钥文件路径")
		}
	default:
		return errors.New("请选择有效的认证方式")
	}
	return nil
}
