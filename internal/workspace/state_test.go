package workspace

import (
	"encoding/json"
	"math"
	"testing"
)

func TestFourPanesEightSessionsMoveMergeAndClose(t *testing.T) {
	s := New()
	var ids []int
	for p := 0; p < 4; p++ {
		if p > 0 {
			axis := "horizontal"
			if p%2 == 0 {
				axis = "vertical"
			}
			if err := s.Split(axis); err != nil {
				t.Fatal(err)
			}
		}
		for i := 0; i < 2; i++ {
			id, err := s.Add("same-profile")
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
	}
	if len(s.Panes()) != 4 {
		t.Fatal("missing panes")
	}
	before, _ := Encode(s)
	if err := s.Split("vertical"); err == nil {
		t.Fatal("fifth pane accepted")
	}
	after, _ := Encode(s)
	if string(after) != string(before) {
		t.Fatal("rejected split mutated layout")
	}
	first, second := ids[0], ids[1]
	s.Close(first)
	if pane, tab := s.Locate(second); tab == nil || pane.Active != second {
		t.Fatal("closing sibling removed or deactivated surviving session")
	}
	_, tab := s.Locate(second)
	tab.FontSize = 17
	lastPane := s.Current().ID
	if err := s.Move(second, lastPane); err != nil {
		t.Fatal(err)
	}
	if pane, tab := s.Locate(second); tab == nil || pane.ID != lastPane || tab.FontSize != 17 || pane.Active != second {
		t.Fatal("moving lost identity, selection or font")
	}
	for len(s.Panes()) > 1 {
		if !s.Merge() {
			t.Fatal("merge failed")
		}
		if err := s.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.Current().Tabs) != 7 {
		t.Fatal("merge lost sessions")
	}
	for _, id := range ids[1:] {
		if _, tab := s.Locate(id); tab == nil {
			t.Fatal("session disappeared")
		}
	}
	for _, id := range ids[1:] {
		s.Close(id)
	}
	if len(s.Current().Tabs) != 0 || s.Current().Active != 0 {
		t.Fatal("last close did not empty pane")
	}
	if s.Merge() {
		t.Fatal("merged only pane")
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPruneDeletedReferencesAndKeepCurrentSelection(t *testing.T) {
	s := New()
	deleted, _ := s.Add("deleted")
	kept, _ := s.Add("kept")
	s.Activate(deleted)
	s.Split("vertical")
	alsoKept, _ := s.Add("kept")
	if count := s.Prune(func(id string) bool { return id == "kept" }); count != 1 {
		t.Fatal("wrong prune count")
	}
	if pane, tab := s.Locate(kept); tab == nil || pane.Active != kept {
		t.Fatal("prune lost replacement selection")
	}
	if s.Current().Active != alsoKept {
		t.Fatal("prune stole active pane")
	}
	if _, tab := s.Locate(deleted); tab != nil {
		t.Fatal("deleted profile resurrected")
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidMutationsLeaveStateIntact(t *testing.T) {
	s := New()
	id, _ := s.Add("profile")
	before, _ := Encode(s)
	if err := s.Move(id, 999); err == nil {
		t.Fatal("missing destination accepted")
	}
	if err := s.Split("invalid"); err == nil {
		t.Fatal("invalid axis accepted")
	}
	if s.Activate(999) {
		t.Fatal("missing tab activated")
	}
	s.Close(999)
	after, _ := Encode(s)
	if string(before) != string(after) {
		t.Fatal("failed mutation changed layout")
	}
}

func TestValidateRejectsAmbiguousOrUnsafeLayouts(t *testing.T) {
	mutations := []func(*State){
		func(s *State) { s.Version = 2 }, func(s *State) { s.Root = nil }, func(s *State) { s.ActivePane = 123 },
		func(s *State) { s.Root.Active = 123 }, func(s *State) { s.Root.Tabs[0].FontSize = float32(math.NaN()) },
		func(s *State) { s.Root.Tabs[0].ID = s.Root.ID }, func(s *State) { s.Root.Axis = "vertical" },
		func(s *State) { s.ContentPercent = 100 }, func(s *State) { s.Root.Tabs[0].ProfileID = "" },
		func(s *State) {
			s.Split("horizontal")
			s.Root.Second.Tabs = append(s.Root.Second.Tabs, s.Root.First.Tabs[0])
			s.Root.Second.Active = s.Root.First.Active
		},
	}
	for i, mutate := range mutations {
		s := New()
		s.Add("profile")
		mutate(s)
		if err := s.Validate(); err == nil {
			data, _ := json.Marshal(s)
			t.Fatalf("mutation %d accepted: %s", i, data)
		}
	}
}
