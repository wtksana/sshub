package desktop

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"time"

	"sshub/internal/library"
	"sshub/internal/transfer"
)

type transferTask struct {
	id, tabID   int
	server      string
	transport   *sharedTransport
	request     transfer.Request
	ctx         context.Context
	cancel      context.CancelFunc
	status, err string
	progress    transfer.Progress
	started     time.Time
	batch       *transferBatch
	profile     library.Profile
	jump        *library.Profile
}
type transferBatch struct{ policy transfer.Choice }

const transferHistoryLimit = 256

func (a *app) pruneTransferHistory() {
	ended := 0
	for _, task := range a.transfers {
		if task.cancel == nil {
			ended++
		}
	}
	remove := max(0, ended-transferHistoryLimit)
	a.transfers = slices.DeleteFunc(a.transfers, func(task *transferTask) bool {
		if remove > 0 && task.cancel == nil {
			remove--
			return true
		}
		return false
	})
}

type fileConflict struct {
	ctx                context.Context
	server, path, name string
	applyAll           bool
	reply              chan transfer.Decision
}

func (a *app) hasTransfers() bool {
	for _, task := range a.transfers {
		if task.cancel != nil {
			return true
		}
	}
	return false
}
func (a *app) hasTabTransfers(id int) bool {
	for _, task := range a.transfers {
		if task.tabID == id && task.cancel != nil {
			return true
		}
	}
	return false
}
func (a *app) cancelTabTransfers(id int) {
	for _, task := range a.transfers {
		if task.tabID == id && task.cancel != nil {
			a.cancelTransfer(task)
		}
	}
}

func (a *app) cancelTransfer(task *transferTask) {
	if task.cancel != nil {
		task.status = "取消中"
		task.cancel()
	}
}

func (a *app) enqueue(tab *workspace, request transfer.Request, batch *transferBatch) {
	if tab == nil || tab.transport == nil || !tab.transport.retain() {
		a.actionError = "请先连接该服务器"
		return
	}
	a.nextTransfer++
	ctx, cancel := context.WithCancel(a.workCtx)
	task := &transferTask{id: a.nextTransfer, tabID: tab.key, server: tab.profile.Name, transport: tab.transport, request: request, ctx: ctx, cancel: cancel, status: "等待中", batch: batch, profile: tab.profile, jump: tab.jump}
	select {
	case a.transferQueue <- task:
		a.transfers = append(a.transfers, task)
	default:
		cancel()
		task.transport.release()
		a.actionError = "传输队列已满，请等待部分任务完成"
	}
}

func (a *app) upload(tab *workspace, dir string, files []string) {
	batch := &transferBatch{}
	for _, file := range files {
		a.enqueue(tab, transfer.Request{Upload: true, Source: file, Destination: path.Join(dir, filepath.Base(file))}, batch)
	}
}

func (a *app) runTransfers() {
	defer close(a.transferDone)
	for task := range a.transferQueue {
		a.runTransfer(task)
		task.transport.release()
	}
}

func (a *app) retryTransfer(task *transferTask) {
	tab := a.workspaceByID(task.tabID)
	if tab == nil {
		id, err := a.layout.Add(task.profile.ID)
		if err != nil {
			a.actionError = err.Error()
			return
		}
		tab = &workspace{key: id, profile: task.profile, jump: task.jump, status: "待连接"}
		a.setDefaultFontSize(id)
		a.tabs = append(a.tabs, tab)
	}
	if tab.client != nil {
		a.enqueue(tab, task.request, &transferBatch{})
		return
	}
	tab.retryRequests = append(tab.retryRequests, task.request)
	a.connect(tab)
}

func (a *app) runTransfer(task *transferTask) {
	ctx := task.ctx
	defer task.cancel()
	a.dispatch(func() { task.status, task.started = "传输中", time.Now() })
	request := task.request
	request.Conflict = func(ctx context.Context, target string) (transfer.Decision, error) {
		if task.batch != nil && task.batch.policy != 0 {
			return transfer.Decision{Choice: task.batch.policy}, nil
		}
		p := &fileConflict{ctx: ctx, server: task.server, path: target, reply: make(chan transfer.Decision, 1)}
		a.dispatch(func() { a.transferConflict = p; task.status = "等待处理同名文件" })
		defer a.dispatch(func() {
			if a.transferConflict == p {
				a.transferConflict = nil
			}
			task.status = "传输中"
		})
		select {
		case d := <-p.reply:
			if d.ApplyAll && d.Choice != transfer.SaveAs && task.batch != nil {
				task.batch.policy = d.Choice
			}
			return d, nil
		case <-ctx.Done():
			return transfer.Decision{}, ctx.Err()
		}
	}
	last := time.Time{}
	request.Progress = func(p transfer.Progress) {
		if time.Since(last) < 150*time.Millisecond {
			return
		}
		last = time.Now()
		a.dispatch(func() { task.progress = p })
	}
	var progress transfer.Progress
	err := ctx.Err()
	if err == nil {
		files, openErr := task.transport.client.OpenFiles(ctx)
		err = openErr
		if err == nil {
			progress, err = transfer.Run(ctx, files.Client, request)
			files.Close()
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) && err == nil {
		err = ctx.Err()
	}
	canceled := errors.Is(ctx.Err(), context.Canceled)
	a.dispatch(func() {
		task.cancel = nil
		task.progress = progress
		task.status = "已完成"
		if err != nil {
			task.status, task.err = "失败", err.Error()
			if canceled {
				task.status = "已取消"
			}
		}
		if tab := a.workspaceByID(task.tabID); tab != nil && tab.client != nil {
			a.browse(tab, tab.files.path)
		}
		a.pruneTransferHistory()
	})
}

func bytesText(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
}
