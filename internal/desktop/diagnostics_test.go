package desktop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiagnosticsAreOptInAndNeverOverwrite(t *testing.T) {
	t.Setenv("SSHUB_DIAGNOSTICS", "")
	if openDiagnostics() != nil {
		t.Fatal("diagnostics enabled by default")
	}
	path := filepath.Join(t.TempDir(), "diagnostics.jsonl")
	t.Setenv("SSHUB_DIAGNOSTICS", path)
	d := openDiagnostics()
	if d == nil {
		t.Fatal("diagnostics missing")
	}
	d.ready = time.Now()
	a := &app{diagnostics: d}
	a.sampleDiagnostics()
	close(d.queue)
	<-d.done
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var sample diagnosticSample
	if err = json.Unmarshal(data, &sample); err != nil || sample.ReadyAt.IsZero() {
		t.Fatal("missing readiness sample", err)
	}
	if openDiagnostics() != nil {
		t.Fatal("diagnostics overwrote existing file")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(data) {
		t.Fatal("original data changed")
	}
}

func TestTransferHistoryBoundPreservesActiveAndRecentTasks(t *testing.T) {
	a := &app{}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := range 1000 {
		a.transfers = append(a.transfers, &transferTask{id: i})
	}
	active := &transferTask{id: 1001, cancel: cancel}
	a.transfers = append([]*transferTask{active}, a.transfers...)
	a.pruneTransferHistory()
	if len(a.transfers) != transferHistoryLimit+1 || a.transfers[0] != active || a.transfers[1].id != 1000-transferHistoryLimit || a.transfers[len(a.transfers)-1].id != 999 {
		t.Fatal("history bound discarded active or recent tasks")
	}
}
