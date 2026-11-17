package desktop

import (
	"fmt"
	"log"
	"time"

	layout "sshub/internal/workspace"
)

func (a *app) restoreLayout() {
	store, state, err := layout.Load(a.dir)
	a.layoutStore, a.layout = store, state
	if err != nil {
		a.layoutReadOnly = true
		a.actionError = "无法读取标签布局，原文件已保留；本次布局不会保存。请将数据目录中的 workspace.json 重命名后重启。"
	}
	dropped := a.layout.Prune(func(id string) bool {
		for _, p := range a.profiles {
			if p.ID == id {
				return true
			}
		}
		return false
	})
	if dropped > 0 {
		a.notice = fmt.Sprintf("已跳过 %d 个配置已删除的终端标签", dropped)
	}
	a.leftSize, a.contentPercent, a.hideLeft, a.hideRight = state.LeftSize, state.ContentPercent, state.HideLeft, state.HideRight
	for _, pane := range state.Panes() {
		for _, tab := range pane.Tabs {
			for _, p := range a.profiles {
				if p.ID == tab.ProfileID {
					a.tabs = append(a.tabs, a.newTab(tab.ID, p))
					break
				}
			}
		}
	}
	if current := a.activeTab(); current != nil {
		a.selected = current.profile.ID
	}
	a.layoutQueue, a.layoutDone = make(chan []byte, 1), make(chan struct{})
	go a.writeLayouts()
}

func (a *app) captureLayout() {
	if a.layout == nil || a.layoutReadOnly {
		return
	}
	a.leftSize = max(120, min(1600, a.leftSize))
	a.contentPercent = max(10, min(90, a.contentPercent))
	var clamp func(*layout.Node)
	clamp = func(n *layout.Node) {
		if n.Axis != "" {
			n.Ratio = max(10, min(90, n.Ratio))
			clamp(n.First)
			clamp(n.Second)
		}
	}
	clamp(a.layout.Root)
	a.layout.LeftSize, a.layout.ContentPercent = a.leftSize, a.contentPercent
	a.layout.HideLeft, a.layout.HideRight = a.hideLeft, a.hideRight
	data, err := layout.Encode(a.layout)
	if err != nil {
		a.actionError = err.Error()
		return
	}
	if string(data) == a.layoutSnapshot {
		return
	}
	a.layoutSnapshot = string(data)
	select {
	case <-a.layoutQueue:
	default:
	}
	a.layoutQueue <- data
}

func (a *app) writeLayouts() {
	defer close(a.layoutDone)
	var pending []byte
	var timer *time.Timer
	var tick <-chan time.Time
	save := func() {
		if pending == nil {
			return
		}
		if err := a.layoutStore.Save(pending); err != nil {
			log.Print("保存标签布局失败：", err)
			a.dispatch(func() { a.actionError = "保存标签布局失败，已有文件未被覆盖：" + err.Error() })
		}
		pending = nil
	}
	for {
		select {
		case data, ok := <-a.layoutQueue:
			if !ok {
				if timer != nil {
					timer.Stop()
				}
				save()
				return
			}
			pending = data
			if timer == nil {
				timer = time.NewTimer(250 * time.Millisecond)
			} else {
				timer.Reset(250 * time.Millisecond)
			}
			tick = timer.C
		case <-tick:
			save()
			tick = nil
		}
	}
}

func (a *app) flushLayout() {
	if a.layoutQueue == nil {
		return
	}
	a.captureLayout()
	close(a.layoutQueue)
	<-a.layoutDone
}
