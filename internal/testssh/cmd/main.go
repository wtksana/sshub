//go:build e2e

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"golang.org/x/crypto/ssh/knownhosts"
	"os"
	"path/filepath"
	"sshub/internal/testssh"
	"time"
)

func main() {
	dir := flag.String("dir", "", "isolated fixture directory")
	delay := flag.Duration("sftp-delay", 0, "delay each fixture SFTP write for lifecycle checks")
	flag.Parse()
	if *dir == "" {
		panic("missing fixture directory")
	}
	if err := os.MkdirAll(*dir, 0700); err != nil {
		panic(err)
	}
	s, err := testssh.Start(*dir)
	if err != nil {
		panic(err)
	}
	defer s.Close()
	s.WriteDelay = *delay
	data, _ := json.Marshal(map[string]string{"address": s.Address(), "identity": s.Identity, "encrypted_identity": s.EncryptedIdentity, "known_host": knownhosts.Line([]string{s.Address()}, s.Host.PublicKey())})
	if err := os.WriteFile(filepath.Join(*dir, "server.json"), data, 0600); err != nil {
		panic(err)
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				stats, _ := json.Marshal(map[string]any{"at": time.Now(), "shells": s.ShellCount.Load(), "sent_bytes": s.SentBytes.Load(), "load": s.LoadEnabled.Load()})
				tmp := filepath.Join(*dir, "stats.tmp")
				if os.WriteFile(tmp, stats, 0600) == nil {
					os.Rename(tmp, filepath.Join(*dir, "stats.json"))
				}
			}
		}
	}()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if scanner.Text() == "disconnect" {
			s.Disconnect()
		} else if scanner.Text() == "quit" {
			return
		} else if scanner.Text() == "load on" {
			s.LoadEnabled.Store(true)
		} else if scanner.Text() == "load off" {
			s.LoadEnabled.Store(false)
		}
	}
}
