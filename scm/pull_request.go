package scm

import "time"

// PullRequestStrategy is the strategy to use for creating pull
// requests. This allows you to not overwhelm you CI.
type PullRequestStrategy string

var (
	// None strategy will create all pull requests at the sametime and is the default
	None PullRequestStrategy = "none"
	// Batch strategy will create pull requests in batches
	Batch PullRequestStrategy = "batch"
)

// PullRequestConfig are the options to use for creating pull requests
type PullRequestConfig struct {
	Title       string        `yaml:"title"`
	Description string        `yaml:"description"`
	Delay       time.Duration `yaml:"delay"`
	AutoMerge   bool          `yaml:"auto_merge"`
}

// Sleep for the given Delay duration
func (c PullRequestConfig) Sleep() {
	time.Sleep(c.Delay)
}
