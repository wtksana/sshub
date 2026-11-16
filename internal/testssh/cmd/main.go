//go:build e2e

package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sshub/internal/testssh"
)

func main() {
	dir := flag.String("dir", "", "isolated fixture directory")
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
	data, _ := json.Marshal(map[string]string{"address": s.Address(), "identity": s.Identity, "encrypted_identity": s.EncryptedIdentity})
	if err := os.WriteFile(filepath.Join(*dir, "server.json"), data, 0600); err != nil {
		panic(err)
	}
	var input [1]byte
	os.Stdin.Read(input[:])
}
