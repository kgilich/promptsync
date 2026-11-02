package promptsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchResyncsWhenSourceChanges(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "promptsync.yaml", "version: 1\nsource: master.md\ntargets:\n  - path: CLAUDE.md\n")
	writeTestFile(t, root, "master.md", "one\n")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	events := make(chan WatchEvent, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Watch(ctx, filepath.Join(root, "promptsync.yaml"), SyncOptions{}, 20*time.Millisecond, func(e WatchEvent) { events <- e })
	}()

	wait := func() WatchEvent {
		select {
		case e := <-events:
			return e
		case <-ctx.Done():
			t.Fatal("timed out waiting for watch event")
			return WatchEvent{}
		}
	}
	if e := wait(); e.Err != nil || len(e.Written) != 1 {
		t.Fatalf("initial event = %+v", e)
	}

	// A conflicting manual edit is reported as an error but does not stop the watch.
	writeTestFile(t, root, "CLAUDE.md", "manual\n")
	writeTestFile(t, root, "master.md", "two\n")
	if e := wait(); e.Err == nil {
		t.Fatalf("expected refusal to overwrite modified target, got %+v", e)
	}
	if got := readTestFile(t, root, "CLAUDE.md"); got != "manual\n" {
		t.Fatalf("watch overwrote a modified target: %q", got)
	}

	// Once the conflict is resolved, the next change syncs again.
	if err := os.Remove(filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "master.md", "three\n")
	if e := wait(); e.Err != nil || len(e.Written) != 1 {
		t.Fatalf("event after resolving conflict = %+v", e)
	}
	cancel()
	<-done
	if got := readTestFile(t, root, "CLAUDE.md"); got != "three\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
}
