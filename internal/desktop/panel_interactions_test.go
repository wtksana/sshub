package desktop

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"sshub/internal/library"
	"sshub/internal/workspace"
)

func TestProfileRowsSortEachGroupIndependently(t *testing.T) {
	a := &app{layout: workspace.New(), group: "全部分组", profiles: []library.Profile{
		{ID: "a1", Group: "A", Name: "Zulu", LastUsed: time.Unix(10, 0)},
		{ID: "b1", Group: "B", Name: "Alpha", LastUsed: time.Unix(20, 0)},
		{ID: "a2", Group: "A", Name: "alpha"},
		{ID: "b2", Group: "B", Name: "Zulu"},
	}}
	assertKeys := func(want ...string) {
		t.Helper()
		rows := a.profileRows(library.Filter(a.profiles, "", "", ""))
		var got []string
		for _, row := range rows {
			got = append(got, row.key())
		}
		if !slices.Equal(got, want) {
			t.Fatalf("rows %v, want %v", got, want)
		}
	}
	assertKeys("group:A", "profile:a2", "profile:a1", "group:B", "profile:b1", "profile:b2", "group:")
	a.layout.GroupSort = map[string]string{"A": "recent", "B": "added"}
	assertKeys("group:A", "profile:a1", "profile:a2", "group:B", "profile:b2", "profile:b1", "group:")
}

func TestDirectoryCommandQuotesRemotePath(t *testing.T) {
	got, ok := directoryCommand("/中文 dir/it's $data; `date`")
	if !ok || got != "cd '/中文 dir/it'\\''s $data; `date`'" {
		t.Fatalf("unexpected command: %q", got)
	}
	for _, dir := range []string{"", "relative", "/bad\npath", "/bad\x1bpath", "/bad\x00path", "/bad\xffpath"} {
		if _, ok := directoryCommand(dir); ok {
			t.Errorf("accepted %q", dir)
		}
	}
}

func TestStartupWindowFitsWorkArea(t *testing.T) {
	work := mygo.Rectangle{X: -1920, Y: 40, Width: 1920, Height: 1040}
	if got := startupWindowBounds(3840, 2160, work); got != work {
		t.Errorf("large saved window: got %+v, want %+v", got, work)
	}
	want := mygo.Rectangle{X: -1600, Y: 160, Width: 1280, Height: 800}
	if got := startupWindowBounds(1280, 800, work); got != want {
		t.Errorf("normal saved window: got %+v, want %+v", got, want)
	}
}
