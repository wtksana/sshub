// Package workspace owns pane layout and restorable session references.
package workspace

import (
	"errors"
	"math"
	"slices"
)

type Tab struct {
	ID        int     `json:"id"`
	ProfileID string  `json:"profile_id"`
	FontSize  float32 `json:"font_size"`
}

type Node struct {
	ID     int     `json:"id"`
	Axis   string  `json:"axis,omitempty"`
	Ratio  float32 `json:"ratio,omitempty"`
	First  *Node   `json:"first,omitempty"`
	Second *Node   `json:"second,omitempty"`
	Tabs   []Tab   `json:"tabs,omitempty"`
	Active int     `json:"active,omitempty"`
}

type State struct {
	Version        int     `json:"version"`
	Root           *Node   `json:"root"`
	ActivePane     int     `json:"active_pane"`
	NextID         int     `json:"next_id"`
	LeftSize       float32 `json:"left_size"`
	ContentPercent float32 `json:"content_percent"`
	HideLeft       bool    `json:"hide_left"`
	HideRight      bool    `json:"hide_right"`
	WindowWidth    int     `json:"window_width,omitempty"`
	WindowHeight   int     `json:"window_height,omitempty"`
}

func New() *State {
	return &State{Version: 1, Root: &Node{ID: 1}, ActivePane: 1, NextID: 2, LeftSize: 260, ContentPercent: 66}
}

func (s *State) Panes() []*Node {
	var panes []*Node
	var visit func(*Node)
	visit = func(n *Node) {
		if n == nil {
			return
		}
		if n.Axis == "" {
			panes = append(panes, n)
		} else {
			visit(n.First)
			visit(n.Second)
		}
	}
	visit(s.Root)
	return panes
}

func (s *State) Current() *Node {
	for _, pane := range s.Panes() {
		if pane.ID == s.ActivePane {
			return pane
		}
	}
	return nil
}

func (s *State) Locate(id int) (*Node, *Tab) {
	for _, pane := range s.Panes() {
		for i := range pane.Tabs {
			if pane.Tabs[i].ID == id {
				return pane, &pane.Tabs[i]
			}
		}
	}
	return nil, nil
}

func (s *State) Add(profileID string) (int, error) {
	pane := s.Current()
	if pane == nil || profileID == "" {
		return 0, errors.New("请选择有效窗格与连接配置")
	}
	count := 0
	for _, p := range s.Panes() {
		count += len(p.Tabs)
	}
	if count >= 1024 {
		return 0, errors.New("已达到 1024 个终端标签的资源上限，请先关闭部分标签")
	}
	id := s.NextID
	s.NextID++
	pane.Tabs = append(pane.Tabs, Tab{ID: id, ProfileID: profileID, FontSize: 13})
	pane.Active = id
	return id, nil
}

func (s *State) Activate(id int) bool {
	pane, _ := s.Locate(id)
	if pane == nil {
		return false
	}
	pane.Active, s.ActivePane = id, pane.ID
	return true
}

func (s *State) Close(id int) {
	pane, _ := s.Locate(id)
	if pane == nil {
		return
	}
	pane.Tabs = slices.DeleteFunc(pane.Tabs, func(t Tab) bool { return t.ID == id })
	if pane.Active == id {
		pane.Active = 0
		if len(pane.Tabs) > 0 {
			pane.Active = pane.Tabs[len(pane.Tabs)-1].ID
		}
	}
}

func (s *State) Split(axis string) error {
	if axis != "horizontal" && axis != "vertical" {
		return errors.New("无效分屏方向")
	}
	if len(s.Panes()) >= 4 {
		return errors.New("最多同时显示四个终端窗格")
	}
	pane := s.Current()
	if pane == nil {
		return errors.New("未选择窗格")
	}
	old := *pane
	*pane = Node{ID: s.NextID, Axis: axis, Ratio: 50, First: &old, Second: &Node{ID: s.NextID + 1}}
	s.NextID += 2
	s.ActivePane = pane.Second.ID
	return nil
}

func (s *State) Move(id, destination int) error {
	source, tab := s.Locate(id)
	if source == nil {
		return errors.New("终端标签不存在")
	}
	var target *Node
	for _, pane := range s.Panes() {
		if pane.ID == destination {
			target = pane
		}
	}
	if target == nil {
		return errors.New("目标窗格不存在")
	}
	if source == target {
		s.Activate(id)
		return nil
	}
	snapshot := *tab
	s.Close(id)
	target.Tabs = append(target.Tabs, snapshot)
	target.Active = id
	s.ActivePane = destination
	return nil
}

// Merge moves every tab from the active pane to an adjacent pane, keeping IDs.
func (s *State) Merge() bool {
	if len(s.Panes()) < 2 {
		return false
	}
	var merge func(*Node) bool
	merge = func(n *Node) bool {
		if n == nil || n.Axis == "" {
			return false
		}
		var removed, sibling *Node
		if n.First.ID == s.ActivePane && n.First.Axis == "" {
			removed, sibling = n.First, n.Second
		}
		if n.Second.ID == s.ActivePane && n.Second.Axis == "" {
			removed, sibling = n.Second, n.First
		}
		if removed != nil {
			target := sibling
			for target.Axis != "" {
				target = target.First
			}
			target.Tabs = append(target.Tabs, removed.Tabs...)
			if removed.Active != 0 {
				target.Active = removed.Active
			}
			s.ActivePane = target.ID
			*n = *sibling
			return true
		}
		return merge(n.First) || merge(n.Second)
	}
	return merge(s.Root)
}

// Prune drops references to deleted profiles without resurrecting their settings.
func (s *State) Prune(exists func(string) bool) int {
	dropped := 0
	for _, pane := range s.Panes() {
		ids := append([]Tab(nil), pane.Tabs...)
		for _, tab := range ids {
			if !exists(tab.ProfileID) {
				s.Close(tab.ID)
				dropped++
			}
		}
	}
	return dropped
}

func (s *State) Validate() error {
	invalid := errors.New("工作区布局无效或版本不支持，原文件已保留")
	if s == nil || s.Version != 1 || s.NextID < 2 || !between(s.LeftSize, 120, 1600) || !between(s.ContentPercent, 10, 90) {
		return invalid
	}
	if s.WindowWidth != 0 && (s.WindowWidth < 900 || s.WindowWidth > 16384) {
		return invalid
	}
	if s.WindowHeight != 0 && (s.WindowHeight < 620 || s.WindowHeight > 16384) {
		return invalid
	}
	ids := map[int]bool{}
	panes, tabs := 0, 0
	active := false
	var visit func(*Node, int) bool
	unique := func(id int) bool {
		if id <= 0 || id >= s.NextID || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	visit = func(n *Node, depth int) bool {
		if n == nil || depth > 8 || !unique(n.ID) {
			return false
		}
		if n.Axis != "" {
			return (n.Axis == "horizontal" || n.Axis == "vertical") && len(n.Tabs) == 0 && n.Active == 0 && between(n.Ratio, 10, 90) && visit(n.First, depth+1) && visit(n.Second, depth+1)
		}
		panes++
		if n.ID == s.ActivePane {
			active = true
		}
		if n.First != nil || n.Second != nil {
			return false
		}
		found := n.Active == 0 && len(n.Tabs) == 0
		for _, tab := range n.Tabs {
			tabs++
			if !unique(tab.ID) || tab.ProfileID == "" || len(tab.ProfileID) > 4096 || !between(tab.FontSize, 8, 32) {
				return false
			}
			if tab.ID == n.Active {
				found = true
			}
		}
		return found
	}
	if !visit(s.Root, 0) || panes < 1 || panes > 4 || tabs > 1024 || !active {
		return invalid
	}
	return nil
}

func between(v, lo, hi float32) bool {
	return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) && v >= lo && v <= hi
}
