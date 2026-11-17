package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestFileColumnSortCycleAndRefresh(t *testing.T) {
	dir := t.TempDir()
	stamp := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	entries := []os.FileInfo{}
	for _, item := range []struct {
		name    string
		size    int
		minutes int
		dir     bool
	}{
		{"folder", 0, 5, true}, {"Alpha.txt", 30, 3, false}, {"beta.txt", 10, 1, false}, {"中文.txt", 20, 2, false},
	} {
		target := filepath.Join(dir, item.name)
		var err error
		if item.dir {
			err = os.Mkdir(target, 0700)
		} else {
			err = os.WriteFile(target, make([]byte, item.size), 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
		modified := stamp.Add(time.Duration(item.minutes) * time.Minute)
		if err = os.Chtimes(target, modified, modified); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, info)
	}
	names := func(in []os.FileInfo) []string {
		out := []string{}
		for _, e := range in {
			out = append(out, e.Name())
		}
		return out
	}
	for _, tc := range []struct {
		column    string
		asc, desc []string
	}{
		{"name", []string{"folder", "Alpha.txt", "beta.txt", "中文.txt"}, []string{"folder", "中文.txt", "beta.txt", "Alpha.txt"}},
		{"modified", []string{"folder", "beta.txt", "中文.txt", "Alpha.txt"}, []string{"folder", "Alpha.txt", "中文.txt", "beta.txt"}},
		{"size", []string{"folder", "beta.txt", "中文.txt", "Alpha.txt"}, []string{"folder", "Alpha.txt", "中文.txt", "beta.txt"}},
	} {
		t.Run(tc.column, func(t *testing.T) {
			f := fileBrowser{entries: entries}
			f.toggleSort(tc.column)
			if !slices.Equal(names(f.displayEntries()), tc.asc) {
				t.Fatalf("ascending: %v", names(f.displayEntries()))
			}
			f.applySort()
			if !slices.Equal(names(f.displayEntries()), tc.asc) {
				t.Fatal("refresh lost sort")
			}
			f.toggleSort(tc.column)
			if !slices.Equal(names(f.displayEntries()), tc.desc) {
				t.Fatalf("descending: %v", names(f.displayEntries()))
			}
			f.toggleSort(tc.column)
			if !slices.Equal(names(f.displayEntries()), names(entries)) || f.sort.direction != 0 {
				t.Fatal("third click did not restore default")
			}
			if !slices.Equal(names(entries), []string{"folder", "Alpha.txt", "beta.txt", "中文.txt"}) {
				t.Fatal("sorting mutated source listing")
			}
		})
	}
	f := fileBrowser{entries: entries}
	f.toggleSort("size")
	f.toggleSort("size")
	f.toggleSort("modified")
	if f.sort.column != "modified" || f.sort.direction != 1 {
		t.Fatal("new column did not start ascending")
	}
	f.entries = entries[1:3]
	f.applySort()
	if !slices.Equal(names(f.displayEntries()), []string{"beta.txt", "Alpha.txt"}) {
		t.Fatal("updated directory retained stale rows")
	}
}

func TestFilePanelSummaryAndModifiedTime(t *testing.T) {
	f := fileBrowser{path: "/data"}
	if f.summary() != "0 项" {
		t.Fatal(f.summary())
	}
	f.loading = true
	if f.summary() != "正在读取目录…" {
		t.Fatal(f.summary())
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "test")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 11, 17, 8, 30, 0, 0, time.UTC)
	if err := os.Chtimes(target, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if fileModifiedText(info) != stamp.Local().Format("2006-01-02 15:04") {
		t.Fatal("modification time did not use local time")
	}
	f.entries = []os.FileInfo{info}
	f.loading = false
	if f.summary() != "1 项" {
		t.Fatal(f.summary())
	}
}
