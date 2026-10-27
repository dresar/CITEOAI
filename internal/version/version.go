package version

import "fmt"

var (
	Version = "v0.1.0-dev"
	Commit  = "none"
	Date    = "unknown"
)

func Formatted() string {
	return fmt.Sprintf("CITEOAI %s (commit: %s, date: %s)", Version, Commit, Date)
}
