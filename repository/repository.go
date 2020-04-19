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

// NewRepository returns an intitialized repository
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

// GetMergeable repositories, repositories where a PR was created
func (r Repositories) GetMergeable(scm SCM) Repositories {
	mergeableRepos := make(Repositories, 0, len(r))

	for n := range r {
		if r[n].SCM == scm && r[n].PullRequestOpened && r[n].PullRequestID != 0 {
			mergeableRepos = append(mergeableRepos, r[n])
		}
	}

	return mergeableRepos
}

// GetCloneable repositories
func (r Repositories) GetCloneable(vcs VCS) Repositories {
	cloneableRepos := make(Repositories, 0, len(r))

	for n := range r {
		if r[n].VCS == vcs && !r[n].PullRequestOpened && !r[n].Cloned {
			cloneableRepos = append(cloneableRepos, r[n])
		}
	}

	return cloneableRepos
}

// GetBumpable repositories
func (r Repositories) GetBumpable() Repositories {
	bumpableRepos := make(Repositories, 0, len(r))

	for n := range r {
		if !r[n].PullRequestOpened && !r[n].Bumped && r[n].Cloned {
			bumpableRepos = append(bumpableRepos, r[n])
		}
	}

	return bumpableRepos
}

// GetPushable repositories
func (r Repositories) GetPushable(vcs VCS) Repositories {
	pushableRepos := make(Repositories, 0, len(r))

	for n := range r {
		if r[n].VCS == vcs && !r[n].PullRequestOpened && !r[n].Pushed && r[n].Bumped {
			pushableRepos = append(pushableRepos, r[n])
		}
	}

	return pushableRepos
}

// GetPRable repositories
func (r Repositories) GetPRable(scm SCM) Repositories {
	prAbleRepos := make(Repositories, 0, len(r))

	for n := range r {
		if r[n].SCM == scm && !r[n].PullRequestOpened && r[n].Pushed {
			prAbleRepos = append(prAbleRepos, r[n])
		}
	}

	return prAbleRepos
}

// GetSavable repositories, repositories where a PR was created
func (r Repositories) GetSavable() Repositories {
	savableRepos := make(Repositories, 0, len(r))

	for n := range r {
		if r[n].PullRequestOpened && r[n].PullRequestID != 0 {
			savableRepos = append(savableRepos, r[n])
		}
	}

	return savableRepos
}
