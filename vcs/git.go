package vcs

import (
	"fmt"
	"log"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/ryancurrah/gomodbump/repository"
)

var (
	goModFilename = "go.mod"
	goSumFilename = "go.sum"
)

// GitConfig are the options to use for Git VCS
type GitConfig struct {
	SourceBranch      string `yaml:"source_branch"`
	TargetBranch      string `yaml:"target_branch"`
	CommitMessage     string `yaml:"commit_message"`
	CommitAuthorName  string `yaml:"commit_author_name"`
	CommitAuthorEmail string `yaml:"commit_author_email"`
	Username          string `yaml:"-"`
	Password          string `yaml:"-"`
}

// Git is a version control system supported by gomodbump
type Git struct {
	conf GitConfig
}

// NewGit initializes a new VCS manager
func NewGit(conf GitConfig) *Git {
	return &Git{conf: conf}
}

// GetSourceBranch returns the source branch to use for creating changes
func (g *Git) GetSourceBranch() string {
	return fmt.Sprintf("%s-%s", g.conf.SourceBranch, time.Now().Format("20060102150405"))
}

// GetTargetBranch returns the branch the source branch was checked out from
func (g *Git) GetTargetBranch() string {
	return g.conf.TargetBranch
}

// VCSType returns the VCS type
func (g *Git) VCSType() repository.VCS {
	return repository.Git
}

// Clone all the repos provided and return the ones that successfully cloned
func (g *Git) Clone(workers int, repos repository.Repositories) {
	reposToClone := make(chan *repository.Repository, len(repos))
	done := make(chan bool, len(repos))

	for worker := 1; worker <= workers; worker++ {
		go g.clone(reposToClone, done)
	}

	for n := range repos {
		reposToClone <- repos[n]
	}

	close(reposToClone)

	for range repos {
		<-done
	}
}

// Push changed files
func (g *Git) Push(workers int, repos repository.Repositories) {
	reposToPush := make(chan *repository.Repository, len(repos))
	done := make(chan bool, len(repos))

	for worker := 1; worker <= workers; worker++ {
		go g.push(reposToPush, done)
	}

	for n := range repos {
		reposToPush <- repos[n]
	}

	close(reposToPush)

	for range repos {
		<-done
	}
}

func (g *Git) clone(repos <-chan *repository.Repository, done chan<- bool) {
	for repo := range repos {
		if repo.SourceBranch == "" {
			repo.SourceBranch = g.GetSourceBranch()
		}

		if repo.TargetBranch == "" {
			repo.TargetBranch = g.GetTargetBranch()
		}

		cloneOpts := git.CloneOptions{
			URL:          repo.URL,
			SingleBranch: true,
			Auth: &http.BasicAuth{
				Username: g.conf.Username,
				Password: g.conf.Password,
			},
		}

		gitRepo, err := git.PlainClone(repo.ClonePath(), false, &cloneOpts)
		if err != nil {
			log.Printf("unable to git clone '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		worktree, err := gitRepo.Worktree()
		if err != nil {
			log.Printf("unable to git clone '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(repo.SourceBranch),
			Create: true,
		})
		if err != nil {
			log.Printf("unable to git clone '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		repo.SetCloned(gitRepo)

		done <- true
	}
}

func (g *Git) push(repos <-chan *repository.Repository, done chan<- bool) {
	for repo := range repos {
		_, err := repo.GitRepo.Worktree()
		if err != nil {
			log.Printf("unable to git push '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		worktree, err := repo.GitRepo.Worktree()
		if err != nil {
			log.Printf("unable to push '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		_, err = worktree.Add(goModFilename)
		if err != nil {
			log.Printf("unable to push '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		_, err = worktree.Add(goSumFilename)
		if err != nil {
			log.Printf("unable to push '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		_, err = worktree.Commit(g.conf.CommitMessage, &git.CommitOptions{
			Author: &object.Signature{
				Name:  g.conf.CommitAuthorName,
				Email: g.conf.CommitAuthorEmail,
				When:  time.Now(),
			},
		})
		if err != nil {
			log.Printf("unable to push '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		err = repo.GitRepo.Push(&git.PushOptions{
			Auth: &http.BasicAuth{
				Username: g.conf.Username,
				Password: g.conf.Password,
			},
		})
		if err != nil {
			log.Printf("unable to push '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		repo.SetPushed()
		done <- true
	}
}
