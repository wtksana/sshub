package desktop

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"
	"sshub/internal/connection"
	"sshub/internal/library"
)

type promptReply struct {
	accepted bool
	value    string
	remember bool
}

type connectionPrompt struct {
	kind, name, address, algorithm, fingerprint, hint string
	value                                             string
	remember                                          bool
	ctx                                               context.Context
	reply                                             chan promptReply
}

func (a *app) ask(ctx context.Context, prompt *connectionPrompt) (promptReply, error) {
	prompt.ctx, prompt.reply = ctx, make(chan promptReply, 1)
	select {
	case a.prompts <- prompt:
	case <-ctx.Done():
		return promptReply{}, ctx.Err()
	}
	a.dispatch(func() { a.nextPrompt() })
	select {
	case response := <-prompt.reply:
		if !response.accepted {
			return response, context.Canceled
		}
		return response, ctx.Err()
	case <-ctx.Done():
		a.dispatch(func() {
			if a.prompt == prompt {
				a.prompt = nil
				a.nextPrompt()
			}
		})
		return promptReply{}, ctx.Err()
	}
}

func (a *app) nextPrompt() {
	for a.prompt == nil {
		select {
		case p := <-a.prompts:
			if p.ctx.Err() == nil {
				a.prompt = p
				a.promptGeneration++
			}
		default:
			return
		}
	}
}

func (a *app) answerPrompt(accepted bool) {
	p := a.prompt
	if p == nil {
		return
	}
	p.reply <- promptReply{accepted: accepted, value: p.value, remember: p.remember}
	p.value = ""
	a.prompt = nil
	a.nextPrompt()
}

func (a *app) requestSecret(ctx context.Context, p library.Profile, kind string, saved map[string]string) (string, error) {
	key := a.secretKey(p, kind)
	value, err := a.vault().Get(key)
	if err == nil {
		secret := string(value)
		clear(value)
		return secret, ctx.Err()
	}
	hint := "默认仅用于本次连接。"
	if !secretMissing(err) {
		hint = "系统凭据库暂不可读，可输入本次凭据；不会回退为明文配置。"
	}
	response, err := a.ask(ctx, &connectionPrompt{kind: kind, name: p.Name, address: p.Host, hint: hint})
	if err != nil {
		return "", err
	}
	if response.remember {
		saved[key] = response.value
	}
	return response.value, nil
}

func (a *app) confirmHost(ctx context.Context, host connection.HostIdentity) (bool, error) {
	response, err := a.ask(ctx, &connectionPrompt{kind: "host", address: host.Address, algorithm: host.Algorithm, fingerprint: host.Fingerprint})
	return response.accepted, err
}

func (a *app) promptView(c *ui.Context) {
	p := a.prompt
	if p == nil {
		return
	}
	open := true
	ui.DialogBase(c.Key(fmt.Sprintf("connection-prompt-%d", a.promptGeneration)), &open, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		dialogStyle(c, panel, 510)
		if p.kind == "host" {
			ui.Text(c, "确认服务器身份").FontSize(typeSize(c, 5)).Bold()
			ui.Text(c, p.address).Bold().Wrap()
			ui.Text(c, "主机密钥："+p.algorithm).Wrap()
			ui.Text(c, p.fingerprint).Wrap()
			ui.Text(c, "请通过可信渠道核对指纹。确认后保存信任记录，继续认证。").TextColor(t.TextMuted).Wrap()
		} else {
			label := "服务器密码"
			if p.kind == "passphrase" {
				label = "私钥口令"
			}
			ui.Text(c, "输入"+label).FontSize(typeSize(c, 5)).Bold()
			ui.Text(c, p.name+"（"+p.address+"）").Wrap()
			textInput(c.Key("connection-secret"), &p.value).Password().Label(label).AutoFocus().FillWidth()
			ui.Checkbox(c.Key("remember-secret"), &p.remember, "记住到系统凭据库")
			ui.Text(c, p.hint).TextColor(t.TextMuted).FontSize(typeSize(c, -1)).Wrap()
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("cancel-connection-prompt"), "取消连接").OnClick(func() { a.answerPrompt(false) })
			label := "继续连接"
			if p.kind == "host" {
				label = "信任并继续"
			}
			ui.PrimaryButton(c.Key("accept-connection-prompt"), label).OnClick(func() { a.answerPrompt(true) })
		})
	})
	if !open {
		a.answerPrompt(false)
	}
}
