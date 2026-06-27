package inspect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amasotti/steno/internal/timeline"
)

func sampleEvents() []timeline.Event {
	return []timeline.Event{
		{Seq: 1, Kind: timeline.KindNavigation, Origin: "https://portal.example", Path: "/login"},
		{Seq: 2, Kind: timeline.KindCall, Origin: "https://portal.example", Method: "GET", Path: "/a", Status: 200},
		{Seq: 3, Kind: timeline.KindCall, Origin: "https://api.example", Method: "POST", Path: "/b", Status: 404},
		{Seq: 4, Kind: timeline.KindCall, Origin: "https://api.example", Method: "GET", Path: "/c", Status: 503},
	}
}

func seqs(events []timeline.Event) []int {
	out := make([]int, len(events))
	for i, e := range events {
		out[i] = e.Seq
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFilterOriginSubstringCaseInsensitive(t *testing.T) {
	got := Filter{Origin: "API"}.Apply(sampleEvents())
	if want := []int{3, 4}; !equalInts(seqs(got), want) {
		t.Fatalf("seqs = %v, want %v", seqs(got), want)
	}
}

func TestFilterStatusClass(t *testing.T) {
	got := Filter{Statuses: []string{"4xx", "5xx"}}.Apply(sampleEvents())
	if want := []int{3, 4}; !equalInts(seqs(got), want) {
		t.Fatalf("seqs = %v, want %v", seqs(got), want)
	}
}

func TestFilterStatusExactCode(t *testing.T) {
	got := Filter{Statuses: []string{"404"}}.Apply(sampleEvents())
	if want := []int{3}; !equalInts(seqs(got), want) {
		t.Fatalf("seqs = %v, want %v", seqs(got), want)
	}
}

func TestFilterStatusDropsNavigationEvents(t *testing.T) {
	got := Filter{Statuses: []string{"2xx"}}.Apply(sampleEvents())
	if want := []int{2}; !equalInts(seqs(got), want) {
		t.Fatalf("seqs = %v, want %v (nav must be excluded)", seqs(got), want)
	}
}

func TestFilterComposesOriginAndStatus(t *testing.T) {
	got := Filter{Origin: "api", Statuses: []string{"5xx"}}.Apply(sampleEvents())
	if want := []int{4}; !equalInts(seqs(got), want) {
		t.Fatalf("seqs = %v, want %v", seqs(got), want)
	}
}

func TestFilterEmptyKeepsAll(t *testing.T) {
	got := Filter{}.Apply(sampleEvents())
	if want := []int{1, 2, 3, 4}; !equalInts(seqs(got), want) {
		t.Fatalf("seqs = %v, want %v", seqs(got), want)
	}
}

func TestFindReturnsEventBySeq(t *testing.T) {
	e, ok := Find(sampleEvents(), 3)
	if !ok || e.Status != 404 {
		t.Fatalf("Find(3) = %+v, ok=%v", e, ok)
	}
	if _, ok := Find(sampleEvents(), 99); ok {
		t.Fatal("Find(99) ok=true, want false")
	}
}

func TestReadArtifactResolvesRelativeToRunDir(t *testing.T) {
	runDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runDir, "payloads"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "payloads", "0005-res.txt"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ReadArtifact(runDir, "payloads/0005-res.txt")
	if err != nil {
		t.Fatalf("ReadArtifact returned error: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("artifact = %q", got)
	}

	if _, err := ReadArtifact(runDir, "payloads/missing.txt"); err == nil {
		t.Fatal("ReadArtifact for missing file returned nil error")
	}
}

func TestParseStatusesInvalidToken(t *testing.T) {
	if _, err := ParseStatuses("4xx,nope"); err == nil {
		t.Fatal("ParseStatuses returned nil error for invalid token")
	}
}

func TestParseStatusesValid(t *testing.T) {
	got, err := ParseStatuses("4xx, 503 ,200")
	if err != nil {
		t.Fatalf("ParseStatuses returned error: %v", err)
	}
	if want := []string{"4xx", "503", "200"}; !equalStrings(got, want) {
		t.Fatalf("ParseStatuses = %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
