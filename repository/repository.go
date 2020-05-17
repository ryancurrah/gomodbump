package repository

import (
	"path/filepath"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-git/v5"
)

// SCM is the kind of scm
type SCM string

// BitbucketServer is a scm type
var BitbucketServer SCM = "bitbucketserver"

// VCS is the kind of vcs
type VCS string

// Git is a vcs type
var Git VCS = "git"

// Update is a module that can be updated
type Update struct {
	Module     string
	OldVersion *semver.Version
	NewVersion *semver.Version
}

// Updates is a list of modules that can be updated
type Updates []*Update

// Repository is a VCS repository
type Repository struct {
	Name              string
	URL               string
	BaseDir           string
	Parent            string
	SourceBranch      string
	TargetBranch      string
	SCM               SCM
	VCS               VCS
	GitRepo           *git.Repository `json:"-"`
	Cloned            bool
	Bumped            bool
	Pushed            bool
	PullRequestOpened bool
	Updates           Updates
	PullRequestID     int64
}

// SetCloned repository state
func (r *Repository) SetCloned(gitRepo *git.Repository) {
	r.Cloned = true
	r.GitRepo = gitRepo
}

// SetBumped repository state
func (r *Repository) SetBumped(updates Updates) {
	r.Bumped = true
	r.Updates = updates
}

// SetPushed repository state
func (r *Repository) SetPushed() {
	r.Pushed = true
}

// SetPullRequest repository state
func (r *Repository) SetPullRequest(id int64) {
	r.PullRequestOpened = true
	r.PullRequestID = id
}

// ClonePath returns the string path to clone to
func (r *Repository) ClonePath() string {
	return filepath.Join(r.BaseDir, string(r.SCM), r.Parent, r.Name)
}

// IsMergeable returns true if a PR exists.
func (r *Repository) IsMergeable(scm SCM) bool {
	return r.SCM == scm && r.PullRequestOpened && r.PullRequestID != 0
}

// IsCloneable returns true if the repo VCS matches and does not already have a PR open.
func (r *Repository) IsCloneable(vcs VCS) bool {
	return r.VCS == vcs
}

// IsBumpable returns true if the repo does not have PR open and is cloned.
func (r *Repository) IsBumpable() bool {
	return !r.PullRequestOpened && !r.Bumped && r.Cloned
}

// IsPushable returns true if the repo does not have PR open and had a dependency updated.
func (r *Repository) IsPushable(vcs VCS) bool {
	return r.VCS == vcs && !r.PullRequestOpened && !r.Pushed && r.Bumped
}

// IsPRable returns true if the repo pushed a commit.
func (r *Repository) IsPRable(scm SCM) bool {
	return r.SCM == scm && !r.PullRequestOpened && r.Pushed
}

// IsSavable returns true if a PR is open.
func (r *Repository) IsSavable() bool {
	return r.PullRequestOpened && r.PullRequestID != 0
}

// ResetState resets the repository state to default
func (r *Repository) ResetState() {
	r.Cloned = false
	r.Bumped = false
	r.Pushed = false
	r.PullRequestOpened = false
	r.Updates = nil
	r.SourceBranch = ""
	r.TargetBranch = ""
	r.PullRequestID = 0
}

// NewRepository returns an initialized repository
func NewRepository(name, url, parent string, scm SCM, vcs VCS) *Repository {
	return &Repository{
		Name:   name,
		URL:    url,
		Parent: parent,
		SCM:    scm,
		VCS:    vcs,
	}
}

// Repositories a list of VCS repositories
type Repositories []*Repository

// GetSavable repositories, repositories where a PR was created
func (r Repositories) GetSavable() Repositories {
	savableRepos := make(Repositories, 0, len(r))

	for n := range r {
		if r[n].IsSavable() {
			savableRepos = append(savableRepos, r[n])
		}
	}

	return savableRepos
}
