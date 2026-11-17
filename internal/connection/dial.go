package connection

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Options struct {
	Target, Identity, KnownHosts string
	Auth                         string
	Secret                       func(context.Context, string) (string, error)
	HostKey                      func(context.Context, string, net.Addr, ssh.PublicKey) error
	Jump                         *Options
}

type Client struct {
	client *ssh.Client
	agent  net.Conn
	jump   *Client
	once   sync.Once
}

func Dial(o Options) (*Client, error) { return DialContext(context.Background(), o) }

func DialContext(ctx context.Context, o Options) (*Client, error) {
	if o.Jump != nil && o.Jump.Jump != nil {
		return nil, errors.New("首版只支持单层跳板机")
	}
	user, address, err := targetAddress(o.Target)
	if err != nil {
		return nil, err
	}
	check := o.HostKey
	if check == nil {
		path := o.KnownHosts
		if path == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			path = filepath.Join(home, ".ssh", "known_hosts")
		}
		verify, err := knownhosts.New(path)
		if err != nil {
			return nil, fmt.Errorf("读取主机信任记录：%w", err)
		}
		check = func(_ context.Context, host string, remote net.Addr, key ssh.PublicKey) error {
			return verify(host, remote, key)
		}
	}
	c := &Client{}
	ok := false
	defer func() {
		if !ok {
			c.Close()
		}
	}()
	if o.Jump != nil {
		c.jump, err = DialContext(ctx, *o.Jump)
		if err != nil {
			return nil, fmt.Errorf("跳板机：%w", err)
		}
	}
	// Resolve key material before opening a network connection. Secrets are never
	// retained in Options, persisted or included in the returned error text.
	auth, err := c.authentication(ctx, o)
	if err != nil {
		return nil, err
	}
	if c.agent != nil {
		stopAgent := context.AfterFunc(ctx, func() { c.agent.Close() })
		defer stopAgent()
		// Agent signing is a separate blocking stream from the SSH transport.
		c.agent.SetDeadline(time.Now().Add(30 * time.Second))
	}
	networkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var raw net.Conn
	if c.jump != nil {
		raw, err = c.jump.client.DialContext(networkCtx, "tcp", address)
	} else {
		raw, err = (&net.Dialer{}).DialContext(networkCtx, "tcp", address)
	}
	if err != nil {
		return nil, fmt.Errorf("连接 %s：%w", address, err)
	}
	stopCancel := context.AfterFunc(ctx, func() { raw.Close() })
	defer stopCancel()
	// A timer also covers forwarded channels, whose SetDeadline is unsupported.
	// Interactive identity/password decisions pause the network timeout.
	timer := time.AfterFunc(20*time.Second, func() { raw.Close() })
	defer timer.Stop()
	var handshaking atomic.Bool
	handshaking.Store(true)
	pause := func(fn func() error) error {
		if !handshaking.Load() {
			return fn()
		}
		timer.Stop()
		defer timer.Reset(20 * time.Second)
		if c.agent != nil {
			c.agent.SetDeadline(time.Time{})
		}
		err := fn()
		if c.agent != nil {
			c.agent.SetDeadline(time.Now().Add(30 * time.Second))
		}
		return err
	}
	if o.Auth == "password" {
		auth = ssh.PasswordCallback(func() (password string, err error) {
			err = pause(func() error { password, err = askSecret(ctx, o, "password"); return err })
			return
		})
	}
	sshConn, channels, requests, err := ssh.NewClientConn(raw, address, &ssh.ClientConfig{
		User: user, Auth: []ssh.AuthMethod{auth},
		HostKeyCallback: func(host string, remote net.Addr, key ssh.PublicKey) error {
			return pause(func() error { return check(ctx, host, remote, key) })
		},
	})
	handshaking.Store(false)
	if err != nil {
		raw.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("SSH 握手失败：%w", err)
	}
	c.client = ssh.NewClient(sshConn, channels, requests)
	if c.agent != nil {
		c.agent.SetDeadline(time.Time{})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ok = true
	return c, nil
}

func targetAddress(target string) (string, string, error) {
	user, address, ok := strings.Cut(target, "@")
	if !ok || user == "" || address == "" {
		return "", "", errors.New("目标需要为 user@host:port")
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		if strings.Contains(address, ":") {
			return "", "", fmt.Errorf("请指定端口并用方括号包围 IPv6：%w", err)
		}
		address = net.JoinHostPort(address, "22")
	}
	return user, address, nil
}

func (c *Client) Close() {
	c.once.Do(func() {
		// Close the outer TCP transport before a forwarded channel can block on it.
		if c.jump != nil {
			c.jump.Close()
		}
		if c.client != nil {
			c.client.Close()
		}
		if c.agent != nil {
			c.agent.Close()
		}
	})
}
