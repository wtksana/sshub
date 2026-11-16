package connection

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type HostIdentity struct {
	Address, Algorithm, Fingerprint string
}

type TrustStore struct {
	mu   sync.Mutex
	path string
}

func NewTrustStore(dir string) *TrustStore {
	return &TrustStore{path: filepath.Join(dir, "known_hosts")}
}

func (s *TrustStore) check(host string, remote net.Addr, key ssh.PublicKey) error {
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return &knownhosts.KeyError{}
	}
	verify, err := knownhosts.New(s.path)
	if err != nil {
		return fmt.Errorf("读取主机信任记录失败，未覆盖原文件：%w", err)
	}
	return verify(host, remote, key)
}

func (s *TrustStore) Verify(ctx context.Context, host string, remote net.Addr, key ssh.PublicKey, confirm func(context.Context, HostIdentity) (bool, error)) error {
	s.mu.Lock()
	err := s.check(host, remote, key)
	s.mu.Unlock()
	if err == nil {
		return ctx.Err()
	}
	var keyErr *knownhosts.KeyError
	if !errors.As(err, &keyErr) {
		return err
	}
	if len(keyErr.Want) != 0 {
		return fmt.Errorf("%s 主机指纹已变化，连接被阻止；核实服务器身份后，在已信任主机中更新记录（当前 %s）", host, ssh.FingerprintSHA256(key))
	}
	if confirm == nil {
		return errors.New("未知主机，需要确认指纹")
	}
	accepted, err := confirm(ctx, HostIdentity{Address: host, Algorithm: key.Type(), Fingerprint: ssh.FingerprintSHA256(key)})
	if err != nil {
		return err
	}
	if !accepted {
		return errors.New("已取消信任未知主机")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	err = s.check(host, remote, key)
	if err == nil {
		return nil
	}
	if !errors.As(err, &keyErr) || len(keyErr.Want) != 0 {
		return errors.New("确认期间主机记录发生变化，请重新连接核实")
	}
	data, err := os.ReadFile(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(data) != 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, []byte(knownhosts.Line([]string{host}, key)+"\n")...)
	return s.write(data)
}

func (s *TrustStore) write(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".known-hosts-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), s.path)
}

func (s *TrustStore) Hosts() ([]HostIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var hosts []HostIdentity
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.HasPrefix(fields[0], "@") || strings.HasPrefix(fields[0], "|") || strings.Contains(fields[0], ",") {
			return nil, errors.New("信任记录包含无法在此界面管理的格式，请检查 known_hosts 文件")
		}
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.Join(fields[1:], " ")))
		if err != nil {
			return nil, errors.New("主机信任记录损坏，未修改原文件")
		}
		hosts = append(hosts, HostIdentity{Address: fields[0], Algorithm: key.Type(), Fingerprint: ssh.FingerprintSHA256(key)})
	}
	return hosts, scan.Err()
}

func (s *TrustStore) Forget(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == address {
			continue
		}
		lines = append(lines, line)
	}
	return s.write([]byte(strings.Join(lines, "\n")))
}
