package desktop

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

type editor struct {
	profile library.Profile
	port    string
	auth    string
	err     string
}

func (a *app) beginEdit(p library.Profile) {
	if p.ID == "" {
		p.Port, p.Auth = 22, "agent"
		if strings.HasPrefix(a.group, "分组：") {
			p.Group = strings.TrimPrefix(a.group, "分组：")
		}
	}
	a.draft = editor{profile: p, port: strconv.Itoa(p.Port), auth: authLabel(p.Auth)}
	a.editGeneration++
	a.editing = true
}

func authLabel(auth string) string {
	switch auth {
	case "password":
		return "密码"
	case "private_key":
		return "私钥文件"
	default:
		return "Windows OpenSSH Agent"
	}
}

func (a *app) saveDraft() {
	if a.store == nil {
		return
	}
	p := a.draft.profile
	port, err := strconv.Atoi(strings.TrimSpace(a.draft.port))
	if err != nil {
		a.draft.err = "端口必须为 1–65535 的整数"
		return
	}
	p.Port = port
	switch a.draft.auth {
	case "密码":
		p.Auth, p.IdentityPath = "password", ""
	case "私钥文件":
		p.Auth = "private_key"
	default:
		p.Auth, p.IdentityPath = "agent", ""
	}
	saved, err := a.store.Upsert(p)
	if err != nil {
		a.draft.err = err.Error()
		return
	}
	a.selected, a.editing, a.actionError = saved.ID, false, ""
	a.refresh()
}

func (a *app) editorView(c *ui.Context) {
	ui.DialogBase(c.Key(fmt.Sprintf("profile-editor-%d", a.editGeneration)), &a.editing, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(500).MaxWidthPercent(94).MaxHeightPercent(94).Padding(20).Gap(10).Radius(8).Background(t.Background).Border(1, t.Border)
		title := "新建连接"
		if a.draft.profile.ID != "" {
			title = "编辑连接"
		}
		ui.Text(c, title).FontSize(18).Bold()
		ui.Scroll(c.Key("editor-fields")).MaxHeight(390).Gap(8).Children(func() {
			field := func(key, label, placeholder string, value *string) {
				ui.Field(c, label, func() { ui.TextInput(c.Key(key), value).Placeholder(placeholder).FillWidth().MaxLength(1024) }).FillWidth()
			}
			field("profile-name", "名称", "例如：生产服务器", &a.draft.profile.Name)
			ui.Row(c).Gap(12).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Children(func() { field("profile-host", "地址", "主机名或 IP", &a.draft.profile.Host) })
				ui.Column(c).Width(90).Children(func() { field("profile-port", "端口", "22", &a.draft.port) })
			})
			field("profile-user", "用户名", "例如：root", &a.draft.profile.Username)
			field("profile-group", "分组", "留空表示未分组", &a.draft.profile.Group)
			ui.Field(c, "认证方式", func() {
				ui.Select(c.Key("profile-auth"), &a.draft.auth, []string{"Windows OpenSSH Agent", "密码", "私钥文件"}).FillWidth()
			})
			if a.draft.auth == "私钥文件" {
				field("profile-identity", "私钥路径", `C:\Users\you\.ssh\id_ed25519`, &a.draft.profile.IdentityPath)
			}
			ui.Checkbox(c.Key("profile-favorite"), &a.draft.profile.Favorite, "加入收藏")
		})
		if a.draft.err != "" {
			ui.Text(c, a.draft.err).TextColor(t.Danger).Wrap()
		}
		ui.Text(c, "只保存连接配置，不会立即连接服务器。").FontSize(11).TextColor(t.TextMuted).Wrap()
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("cancel-profile"), "取消").OnClick(func() { a.editing = false })
			ui.PrimaryButton(c.Key("save-profile"), "保存连接").OnClick(a.saveDraft)
		})
	})
}
