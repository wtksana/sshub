package library

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// PortableProfile is an explicit export allowlist. Credentials, trust records,
// recent use and local review state never enter the migration format.
type PortableProfile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Auth         string `json:"auth"`
	IdentityPath string `json:"identity_path,omitempty"`
	Group        string `json:"group,omitempty"`
	Favorite     bool   `json:"favorite"`
	JumpID       string `json:"jump_id,omitempty"`
}

type Migration struct {
	Format      string            `json:"format"`
	Version     int               `json:"version"`
	Profiles    []PortableProfile `json:"profiles"`
	Preferences *Preferences      `json:"preferences,omitempty"`
}

func (p PortableProfile) profile() Profile {
	return Profile{ID: p.ID, Name: strings.TrimSpace(p.Name), Host: strings.TrimSpace(p.Host), Port: p.Port, Username: strings.TrimSpace(p.Username), Auth: p.Auth, IdentityPath: strings.TrimSpace(p.IdentityPath), Group: strings.TrimSpace(p.Group), Favorite: p.Favorite, JumpID: p.JumpID, IdentityNeedsReview: p.Auth == "private_key"}
}

func DecodeMigration(r io.Reader) (*Migration, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxConfigBytes+1))
	if err != nil {
		return nil, errors.New("无法读取迁移文件")
	}
	if len(data) > maxConfigBytes {
		return nil, errors.New("迁移文件超过 8 MiB")
	}
	var m Migration
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return nil, fmt.Errorf("迁移文件格式无效：%w", err)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, errors.New("迁移文件包含多余内容")
	}
	if err := validateMigration(m); err != nil {
		return nil, err
	}
	return &m, nil
}

func validateMigration(m Migration) error {
	if m.Format != "sshub" || m.Version != 1 || m.Profiles == nil {
		return errors.New("不是受支持的 SSHub 迁移文件")
	}
	d := Document{Version: 1, Preferences: m.Preferences}
	names := map[string]bool{}
	for _, p := range m.Profiles {
		v := p.profile()
		key := nameKey(v)
		if names[key] {
			return errors.New("迁移文件中存在同组重名连接，请先修正文件")
		}
		names[key] = true
		d.Profiles = append(d.Profiles, v)
	}
	return validateDocument(d)
}

func nameKey(p Profile) string {
	// Match the same Unicode equivalence as Upsert's strings.EqualFold.
	fold := func(s string) string {
		return strings.Map(func(r rune) rune {
			key := r
			for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
				key = min(key, next)
			}
			return key
		}, strings.TrimSpace(s))
	}
	return fold(p.Group) + "\x00" + fold(p.Name)
}

func (s *Store) Export() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefs := preferences(s.doc)
	m := Migration{Format: "sshub", Version: 1, Preferences: &prefs, Profiles: []PortableProfile{}}
	for _, p := range s.doc.Profiles {
		m.Profiles = append(m.Profiles, PortableProfile{ID: p.ID, Name: p.Name, Host: p.Host, Port: p.Port, Username: p.Username, Auth: p.Auth, IdentityPath: p.IdentityPath, Group: p.Group, Favorite: p.Favorite, JumpID: p.JumpID})
	}
	data, err := json.MarshalIndent(m, "", "  ")
	return append(data, '\n'), err
}

type ImportRow struct {
	Profile  Profile
	Conflict bool
}
type ImportDecision struct {
	Mode string // add, skip, rename, replace
	Name string
}
type ImportPlan struct {
	owner       *Store
	Rows        []ImportRow
	Preferences *Preferences
	base        []byte
	profiles    []Profile
	conflicts   []string
	prefs       *Preferences
}

func (s *Store) PreviewImport(m Migration) (*ImportPlan, error) {
	if err := validateMigration(m); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plan := &ImportPlan{owner: s, base: slices.Clone(s.source)}
	if m.Preferences != nil {
		p := *m.Preferences
		plan.prefs = &p
		copy := p
		plan.Preferences = &copy
	}
	for _, p := range m.Profiles {
		v := p.profile()
		conflict := ""
		for _, current := range s.doc.Profiles {
			if nameKey(current) == nameKey(v) {
				if conflict != "" {
					return nil, errors.New("本地连接存在同组重名，请先整理后再导入")
				}
				conflict = current.ID
			}
		}
		plan.profiles = append(plan.profiles, v)
		plan.conflicts = append(plan.conflicts, conflict)
		plan.Rows = append(plan.Rows, ImportRow{Profile: v, Conflict: conflict != ""})
	}
	return plan, nil
}

type ImportResult struct {
	Count       int
	ReplacedIDs map[string]string
}

// CommitImport validates the entire resulting graph, then uses one reliable
// write for profiles and preferences. Imported identities are always fresh.
func (s *Store) CommitImport(plan *ImportPlan, decisions []ImportDecision, applyPreferences bool) (ImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if plan == nil || plan.owner != s || len(decisions) != len(plan.profiles) {
		return ImportResult{}, errors.New("导入决策不完整，请重新预览")
	}
	if !bytes.Equal(plan.base, s.source) {
		return ImportResult{}, errors.New("连接或偏好已变化，请重新预览后导入")
	}
	next := Document{Version: 1, Profiles: slices.Clone(s.doc.Profiles), Preferences: s.doc.Preferences}
	ids, replaced := map[string]string{}, map[string]string{}
	for i, p := range plan.profiles {
		d := decisions[i]
		if d.Mode == "skip" {
			if plan.conflicts[i] != "" {
				ids[p.ID] = plan.conflicts[i]
			}
			continue
		}
		if d.Mode != "add" && d.Mode != "rename" && d.Mode != "replace" {
			return ImportResult{}, errors.New("请处理全部重名冲突")
		}
		if d.Mode == "add" && plan.conflicts[i] != "" {
			return ImportResult{}, errors.New("同组重名连接需要选择跳过、另存或替换")
		}
		if d.Mode == "replace" && plan.conflicts[i] == "" {
			return ImportResult{}, errors.New("替换目标已不存在，请重新预览")
		}
		id := rand.Text()
		ids[p.ID] = id
		if d.Mode == "replace" {
			replaced[plan.conflicts[i]] = id
		}
	}
	next.Profiles = slices.DeleteFunc(next.Profiles, func(p Profile) bool { return replaced[p.ID] != "" })
	count := 0
	for i, original := range plan.profiles {
		d := decisions[i]
		if d.Mode == "skip" {
			continue
		}
		p := original
		p.ID = ids[original.ID]
		if p.JumpID != "" {
			mapped := ids[p.JumpID]
			if mapped == "" {
				return ImportResult{}, errors.New("所选连接引用了被跳过的跳板机，请一并导入或保留同名跳板机")
			}
			p.JumpID = mapped
		}
		if d.Mode == "rename" {
			p.Name = strings.TrimSpace(d.Name)
		}
		if err := Validate(p); err != nil {
			return ImportResult{}, err
		}
		for _, current := range next.Profiles {
			if nameKey(current) == nameKey(p) {
				return ImportResult{}, errors.New("另存名称仍与现有连接重名，请更换名称")
			}
		}
		next.Profiles = append(next.Profiles, p)
		count++
	}
	for i := range next.Profiles {
		if id := replaced[next.Profiles[i].JumpID]; id != "" {
			next.Profiles[i].JumpID = id
		}
	}
	if applyPreferences && plan.prefs != nil {
		p := *plan.prefs
		next.Preferences = &p
	}
	if err := validateDocument(next); err != nil {
		return ImportResult{}, err
	}
	if err := s.save(next); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Count: count, ReplacedIDs: replaced}, nil
}

// ExportTo never writes into files managed by this application's data store.
func (s *Store) ExportTo(destination string, overwrite bool) error {
	data, err := s.Export()
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	targetInfo, targetErr := os.Stat(abs)
	if targetErr != nil && !os.IsNotExist(targetErr) {
		return targetErr
	}
	if targetErr == nil && !overwrite {
		return errors.New("目标文件已存在，请确认覆盖")
	}
	parent, parentErr := os.Stat(filepath.Dir(abs))
	dataDir, dataDirErr := os.Stat(filepath.Dir(s.path))
	for _, name := range []string{"connections.json", "workspace.json", "known_hosts", "library.lock"} {
		managed := filepath.Join(filepath.Dir(s.path), name)
		if strings.EqualFold(abs, managed) || (strings.EqualFold(filepath.Base(abs), name) && parentErr == nil && dataDirErr == nil && os.SameFile(parent, dataDir)) {
			return errors.New("不能覆盖应用的数据文件，请选择其他位置")
		}
		info, err := os.Stat(managed)
		if err == nil && targetErr == nil && os.SameFile(info, targetInfo) {
			return errors.New("不能覆盖应用的数据文件，请选择其他位置")
		}
	}
	f, err := os.CreateTemp(filepath.Dir(abs), ".sshub-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), abs)
}
