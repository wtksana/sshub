package connection

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func askSecret(ctx context.Context, o Options, kind string) (string, error) {
	if o.Secret == nil {
		return "", errors.New("需要提供密码或私钥口令")
	}
	return o.Secret(ctx, kind)
}

func (c *Client) authentication(ctx context.Context, o Options) (ssh.AuthMethod, error) {
	auth := o.Auth
	if auth == "" {
		auth = "agent"
		if o.Identity != "" {
			auth = "private_key"
		}
	}
	switch auth {
	case "password":
		return ssh.PasswordCallback(func() (string, error) { return askSecret(ctx, o, "password") }), nil
	case "private_key":
		key, err := os.ReadFile(o.Identity)
		if err != nil {
			return nil, fmt.Errorf("读取私钥：%w", err)
		}
		defer clear(key)
		signer, err := ssh.ParsePrivateKey(key)
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			secret, askErr := askSecret(ctx, o, "passphrase")
			if askErr != nil {
				return nil, askErr
			}
			phrase := []byte(secret)
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, phrase)
			clear(phrase)
		}
		if err != nil {
			return nil, errors.New("无法解锁或解析私钥，请检查文件与口令")
		}
		return ssh.PublicKeys(signer), nil
	case "agent":
		agentCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		var err error
		c.agent, err = dialAgent(agentCtx)
		if err != nil {
			return nil, fmt.Errorf("无法访问 SSH Agent，请确认服务已启动且已载入密钥：%w", err)
		}
		return ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			signers, err := agent.NewClient(c.agent).Signers()
			return signers, err
		}), nil
	default:
		return nil, errors.New("不支持的认证方式")
	}
}
