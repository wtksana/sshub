package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo"
	"sshub/internal/library"
)

func (a *app) secretKey(p library.Profile, kind string) string {
	dir, _ := filepath.Abs(a.dir)
	identity := dir + "\x00" + p.ID + "\x00" + p.Username + "\x00" + net.JoinHostPort(p.Host, strconv.Itoa(p.Port)) + "\x00" + p.Auth + "\x00" + p.IdentityPath + "\x00" + kind
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}

func (a *app) vault() mygo.SecretsModule { return mygo.Secrets.Service("dev.sshub.credentials") }

func (a *app) forgetCredentials(p library.Profile) {
	go func() {
		var first error
		for _, kind := range []string{"password", "passphrase"} {
			if err := a.vault().Delete(a.secretKey(p, kind)); err != nil && first == nil {
				first = err
			}
		}
		a.dispatch(func() {
			if first != nil {
				a.actionError = "清除系统凭据失败：" + first.Error()
			} else {
				a.notice = fmt.Sprintf("已删除“%s”的已保存凭据", p.Name)
			}
		})
	}()
}

func secretMissing(err error) bool { return errors.Is(err, mygo.ErrSecretNotFound) }
