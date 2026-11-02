package promptsync

import (
	"context"
	"crypto/sha256"
	"time"
)

// WatchEvent reports the outcome of one watch cycle.
type WatchEvent struct {
	Written []string
	Err     error
}

// Watch syncs once, then re-syncs whenever the generated output changes
// (source, rule, template, variable, or config edits) until ctx is cancelled.
// It polls rather than using OS file notifications so it behaves the same on
// every platform and needs no dependencies. Errors, including refusals to
// overwrite modified targets, are reported once per distinct failure and do
// not stop the watch, so a half-saved file never kills the loop.
func Watch(ctx context.Context, configPath string, options SyncOptions, interval time.Duration, report func(WatchEvent)) error {
	var lastFingerprint [sha256.Size]byte
	var lastErr string
	first := true

	cycle := func() {
		targets, _, _, err := generateTargets(configPath)
		if err != nil {
			if err.Error() != lastErr {
				lastErr = err.Error()
				report(WatchEvent{Err: err})
			}
			return
		}
		hash := sha256.New()
		for _, target := range targets {
			hash.Write([]byte(target.path))
			hash.Write([]byte{0})
			hash.Write(target.content)
			hash.Write([]byte{0})
		}
		var fingerprint [sha256.Size]byte
		copy(fingerprint[:], hash.Sum(nil))
		if !first && fingerprint == lastFingerprint {
			return
		}

		written, err := SyncWithOptions(configPath, options)
		if err != nil {
			if err.Error() != lastErr {
				lastErr = err.Error()
				report(WatchEvent{Err: err})
			}
			// Retry only once the output changes again.
			lastFingerprint, first = fingerprint, false
			return
		}
		lastErr = ""
		lastFingerprint, first = fingerprint, false
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
