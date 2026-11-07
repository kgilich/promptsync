package promptsync

import (
	"context"
	"crypto/sha256"
	"os"
	"time"
)

// WatchEvent reports the outcome of one watch cycle.
type WatchEvent struct {
	Written []string
	Err     error
}

// Watch syncs once, then re-syncs whenever the generated output or the state
// of the target files on disk changes (source, rule, template, variable, or
// config edits; a conflicting target being fixed or deleted) until ctx is
// cancelled. It polls rather than using OS file notifications so it behaves
// the same on every platform and needs no dependencies. Errors, including
// refusals to overwrite modified targets, are reported once per distinct
// failure and do not stop the watch, so a half-saved file never kills the loop.
func Watch(ctx context.Context, configPath string, options SyncOptions, interval time.Duration, report func(WatchEvent)) error {
	var last [sha256.Size]byte
	var lastErr string
	first := true

	fail := func(err error) {
		if err.Error() != lastErr {
			lastErr = err.Error()
			report(WatchEvent{Err: err})
		}
	}

	cycle := func() {
		targets, root, _, err := generateTargets(configPath)
		if err != nil {
			fail(err)
			return
		}
		lastErr = "" // inputs are valid again; a repeat of an old failure must be reported
		current := fingerprint(targets, root)
		if !first && current == last {
			return
		}

		written, err := SyncWithOptions(configPath, options)
		first = false
		if err != nil {
			// Retry only once the generated output or the files on disk change.
			last = current
			fail(err)
			return
		}
		last = fingerprint(targets, root) // include what the sync itself just wrote
		report(WatchEvent{Written: written})
	}

	cycle()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			cycle()
		}
	}
}

// fingerprint hashes the generated content together with what is currently on
// disk at each target path.
func fingerprint(targets []generatedTarget, root string) [sha256.Size]byte {
	hash := sha256.New()
	for _, target := range targets {
		hash.Write([]byte(target.path))
		hash.Write([]byte{0})
		hash.Write(target.content)
		hash.Write([]byte{0})
		if existing, err := os.ReadFile(resolve(root, target.path)); err == nil {
			hash.Write([]byte{1})
			hash.Write(existing)
		} else {
			hash.Write([]byte{2})
		}
		hash.Write([]byte{0})
	}
	var sum [sha256.Size]byte
	copy(sum[:], hash.Sum(nil))
	return sum
}
