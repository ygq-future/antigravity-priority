package state

import (
	"path/filepath"
	"strings"

	"antigravity-priority/internal/config"
)

// Paths identifies the split cache documents inside the plugin's cache directory.
// Main holds quota evidence and health state; sibling documents isolate run
// history and high-frequency request history so writes never rewrite unrelated
// state.
type Paths struct {
	Main     string
	Runtime  string
	Runs     string
	Requests string
}

// ResolvePaths derives the cache documents from the configured main path.
// The legacy default path is redirected into the dedicated
// "data/antigravity-priority" directory, and the default "state.json" uses
// clean sibling names so the directory stays self-describing.
func ResolvePaths(main string) Paths {
	return buildPaths(normalizeMainPath(main), true)
}

// LegacyPaths returns the pre-directory document layout for a configured path.
// It is used only to migrate and remove old files; new writes always target
// ResolvePaths. The new default still maps back to the legacy default so an
// untouched installation migrates on first start.
func LegacyPaths(main string) Paths {
	main = strings.TrimSpace(main)
	if main == "" ||
		filepath.Clean(main) == filepath.Clean(config.DefaultStateCachePath) ||
		filepath.Clean(main) == filepath.Clean(config.LegacyDefaultStateCachePath) {
		main = config.LegacyDefaultStateCachePath
	}
	return buildPaths(main, false)
}

func normalizeMainPath(main string) string {
	main = strings.TrimSpace(main)
	if main == "" || filepath.Clean(main) == filepath.Clean(config.LegacyDefaultStateCachePath) {
		return config.DefaultStateCachePath
	}
	return main
}

func buildPaths(main string, cleanNames bool) Paths {
	dir := filepath.Dir(main)
	base := strings.TrimSuffix(filepath.Base(main), filepath.Ext(main))
	if cleanNames && base == "state" {
		return Paths{
			Main:     main,
			Runtime:  filepath.Join(dir, "runtime.json"),
			Runs:     filepath.Join(dir, "runs.json"),
			Requests: filepath.Join(dir, "requests.json"),
		}
	}
	return Paths{
		Main:     main,
		Runtime:  filepath.Join(dir, base+".runtime.json"),
		Runs:     filepath.Join(dir, base+".runs.json"),
		Requests: filepath.Join(dir, base+".requests.json"),
	}
}
