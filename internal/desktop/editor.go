package desktop

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"sshub/internal/library"
)

type editor struct {
	identityReviewed bool
	profile          library.Profile
	port             string
	auth             string
	err              string
	advanced         bool
	jumpLabel        string
}

func (a *app) beginEdit(p library.Profile) {
	if p.ID == "" {
		p.Port, p.Auth = 22, "password"
		if strings.HasPrefix(a.group, "分组：") {
			p.Group = strings.TrimPrefix(a.group, "分组：")
		}
	}
	a.startEditor(p)
}

func (a *app) beginCopy(p library.Profile) {
	name := p.Name + " 副本"
	for n := 2; ; n++ {
		found := false
		for _, existing := range a.profiles {
			if strings.EqualFold(existing.Group, p.Group) && strings.EqualFold(existing.Name, name) {
				found = true
				break
			}
		}
		if !found {
			break
		}
		name = fmt.Sprintf("%s 副本 %d", p.Name, n)
	}
	// Copy connection details explicitly, never the original credential identity.
	a.startEditor(library.Profile{Name: name, Host: p.Host, Port: p.Port, Username: p.Username, Group: p.Group, Favorite: p.Favorite, JumpID: p.JumpID, Auth: "password"})
}

func (a *app) startEditor(p library.Profile) {
	a.draft = editor{profile: p, port: strconv.Itoa(p.Port), auth: authLabel(p.Auth)}
	a.draft.advanced = p.JumpID != ""
	a.draft.jumpLabel = "不使用跳板机"
	for _, v := range a.profiles {
		if v.ID == p.JumpID {
			a.draft.jumpLabel = "经由：" + v.Name
		}
	}
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
	p.JumpID = ""
	if a.draft.jumpLabel != "不使用跳板机" {
		for _, v := range a.profiles {
			if a.draft.jumpLabel == "经由："+v.Name {
				p.JumpID = v.ID
			}
		}
		if p.JumpID == "" {
			a.draft.err = "跳板机配置已不存在，请重新选择"
			return
		}
	}
	switch a.draft.auth {
	case "密码":
		p.Auth, p.IdentityPath = "password", ""
	case "私钥文件":
		p.Auth = "private_key"
	default:
		p.Auth, p.IdentityPath = "agent", ""
	}
	if p.Auth == "private_key" && p.IdentityNeedsReview && !a.draft.identityReviewed {
		a.draft.err = "请核对本机私钥路径并勾选确认"
		return
	}
	p.IdentityNeedsReview = false
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
		dialogStyle(c, panel, 500)
		title := "新建连接"
		if a.draft.profile.ID != "" {
			title = "编辑连接"
		}
		ui.Text(c, title).FontSize(typeSize(c, 5)).Bold()
		ui.Scroll(c.Key("editor-fields")).MaxHeight(390).Gap(8).Children(func() {
			field := func(key, label, placeholder string, value *string) {
				formField(c, label, func() { textInput(c.Key(key), value).Placeholder(placeholder).FillWidth().MaxLength(1024) }).FillWidth()
			}
			field("profile-name", "名称", "例如：生产服务器", &a.draft.profile.Name)
			ui.Row(c).Gap(12).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Children(func() { field("profile-host", "地址", "主机名或 IP", &a.draft.profile.Host) })
				ui.Column(c).Width(90).Children(func() { field("profile-port", "端口", "22", &a.draft.port) })
			})
			field("profile-user", "用户名", "例如：root", &a.draft.profile.Username)
			formField(c, "分组", func() { a.groupInput(c, "profile-group", &a.draft.profile.Group) })
			formField(c, "认证方式", func() {
				ui.Select(c.Key("profile-auth"), &a.draft.auth, []string{"密码", "私钥文件", "Windows OpenSSH Agent"}).FillWidth().MinHeight(36).Radius(6)
			})
			if a.draft.auth == "私钥文件" {
				field("profile-identity", "私钥路径", `C:\Users\you\.ssh\id_ed25519`, &a.draft.profile.IdentityPath)
				if a.draft.profile.IdentityNeedsReview {
					ui.Text(c, "此路径来自导入文件，连接前需要重新核对。").Wrap()
					ui.Checkbox(c.Key("identity-reviewed"), &a.draft.identityReviewed, "已核对本机私钥路径")
				}
			}
			ui.Checkbox(c.Key("profile-favorite"), &a.draft.profile.Favorite, "加入收藏")
			ui.Checkbox(c.Key("profile-advanced"), &a.draft.advanced, "高级选项")
			if a.draft.advanced {
				choices := []string{"不使用跳板机"}
				for _, v := range a.profiles {
					if v.ID != a.draft.profile.ID && v.JumpID == "" {
						choices = append(choices, "经由："+v.Name)
					}
				}
				formField(c, "单层跳板机", func() {
					ui.Select(c.Key("profile-jump"), &a.draft.jumpLabel, choices).FillWidth().MinHeight(36).Radius(6)
				})
			}
		})
		if a.draft.err != "" {
			ui.Text(c, a.draft.err).TextColor(t.Danger).Wrap()
		}
		ui.Text(c, "只保存连接配置，不会立即连接服务器。").FontSize(typeSize(c, -2)).TextColor(t.TextMuted).Wrap()
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("cancel-profile"), "取消").OnClick(func() { a.editing = false })
			ui.PrimaryButton(c.Key("save-profile"), "保存连接").OnClick(a.saveDraft)
		})
	})
}
