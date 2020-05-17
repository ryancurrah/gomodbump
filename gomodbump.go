package gomodbump

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/ryancurrah/gomodbump/bump"
	"github.com/ryancurrah/gomodbump/repository"
	"github.com/ryancurrah/gomodbump/scm"
	"github.com/ryancurrah/gomodbump/storage"
	"github.com/ryancurrah/gomodbump/vcs"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type scmManager interface {
	SCMType() repository.SCM
	GetRepositories(vcsType repository.VCS) (repository.Repositories, error)
	MergePullRequest(repo *repository.Repository) error
	CreatePullRequest(repo *repository.Repository) (int, error)
}

type vcsManager interface {
	GetSourceBranch() string
	GetTargetBranch() string
	VCSType() repository.VCS
	Clone(repo *repository.Repository) (*git.Repository, error)
	Push(repo *repository.Repository) error
	DeleteBranch(repo *repository.Repository) error
}

type bumper interface {
	Bump(repo *repository.Repository) (repository.Updates, error)
}

type storageManager interface {
	Save(repos repository.Repositories) error
	Load() (repository.Repositories, error)
}

// GeneralConfig are general settings for this package.
type GeneralConfig struct {
	Workers   int           `yaml:"workers"`
	WorkDir   string        `yaml:"work_dir"`
	CloneType string        `yaml:"clone_type"`
	Stateful  bool          `yaml:"stateful"`
	Cleanup   bool          `yaml:"cleanup"`
	Delay     time.Duration `yaml:"delay"`
}

// SourceCodeManagementConfig used to create pull requests and get repos.
type SourceCodeManagementConfig struct {
	PullRequest     scm.PullRequestConfig     `yaml:"pull_request"`
	BitbucketServer scm.BitbucketServerConfig `yaml:"bitbucket_server"`
}

// VersionControlSystemConfig used to work with the repos.
type VersionControlSystemConfig struct {
	Git vcs.GitConfig `yaml:"git"`
}

// StorageConfig allow different file storage backends.
type StorageConfig struct {
	File storage.FileStorageConfig `yaml:"file"`
	S3   storage.S3StorageConfig   `yaml:"s3"`
}

// Configuration for Go mod bump.
type Configuration struct {
	General GeneralConfig              `yaml:"general"`
	SCM     SourceCodeManagementConfig `yaml:"scm"`
	VCS     VersionControlSystemConfig `yaml:"vcs"`
	Bump    bump.Configuration         `yaml:"bump"`
	Storage StorageConfig              `yaml:"storage"`
}

// GetWorkDir returns the working dir path cleaned.
func (c Configuration) GetWorkDir() string {
	return filepath.Clean(c.General.WorkDir)
}

// GoModBump does the bumping of versions.
type GoModBump struct {
	conf           Configuration
	scmManager     scmManager
	vcsManager     vcsManager
	bumper         bumper
	storageManager storageManager
}

// NewGoModBump initializes a Go Mod Bump struct.
func NewGoModBump(conf Configuration) (*GoModBump, error) {
	vcsManager, err := vcs.NewGit(conf.VCS.Git, conf.General.CloneType)
	if err != nil {
		return nil, err
	}

	var storageManager storageManager

	if conf.Storage.S3 != (storage.S3StorageConfig{}) {
		var errStorage error

		storageManager, errStorage = storage.NewS3Storage(conf.Storage.S3)
		if errStorage != nil {
			return nil, errStorage
		}
	} else {
		storageManager = storage.NewFileStorage(conf.Storage.File)
	}

	return &GoModBump{
		conf:           conf,
		scmManager:     scm.NewBitbucketServer(conf.SCM.PullRequest, conf.SCM.BitbucketServer, conf.General.CloneType),
		vcsManager:     vcsManager,
		bumper:         bump.NewBumper(conf.Bump),
		storageManager: storageManager,
	}, nil
}

// Run Go Mod Bump.
func (b *GoModBump) Run() error { // nolint:gocognit
	ctx := context.Background()

	// Cleanup working directory before running.
	b.clean()

	// Get the repos from the last the run, this contains PR info.
	reposFromStorage, err := b.storageManager.Load()
	if err != nil {
		return err
	}

	// Get current repos from SCM.
	reposFromSCM, err := b.scmManager.GetRepositories(b.vcsManager.VCSType())
	if err != nil {
		return err
	}

	// Converge the repos from storage into the repos from SCM.
	repos := converge(b.conf.GetWorkDir(), reposFromStorage, reposFromSCM)

	sem := semaphore.NewWeighted(int64(b.conf.General.Workers))

	group, ctx := errgroup.WithContext(ctx)

	for n := range repos {
		repo := repos[n]

		group.Go(func() error {
			err := sem.Acquire(ctx, 1)
			if err != nil {
				return err
			}
			defer sem.Release(1)

			// Clone repos locally.
			if repo.IsCloneable(b.vcsManager.VCSType()) {
				vcsRepoClient, err := b.vcsManager.Clone(repo)
				if err != nil {
					return err
				}

				repo.SetCloned(vcsRepoClient)
			}

			// If any of the repos have a pull request open and they are mergeable, merge them (If auto_merge=true).
			if repo.IsMergeable(b.scmManager.SCMType()) {
				err = b.scmManager.MergePullRequest(repo)
				if err != nil {
					return err
				}

				err = b.vcsManager.DeleteBranch(repo)
				if err != nil {
					return err
				}

				repo.ResetState()

				log.Printf("repo '%s': merged pull request and sleeping for %v", repo.Name, b.conf.General.Delay)

				b.sleep()
			}

			// Find and update Go module dependencies.
			if repo.IsBumpable() {
				updates, err := b.bumper.Bump(repo)
				if err != nil {
					return err
				}

				if updates == nil {
					return nil
				}

				repo.SetBumped(updates)
			}

			// Push repos to remote, includes committing.
			if repo.IsPushable(b.vcsManager.VCSType()) {
				err = b.vcsManager.Push(repo)
				if err != nil {
					return err
				}

				repo.SetPushed()

				log.Printf("repo '%s': pushed and sleeping for %v", repo.Name, b.conf.General.Delay)

				b.sleep()
			}

			// Create pull requests for repos where they are PRable.
			if repo.IsPRable(b.scmManager.SCMType()) {
				pullRequestID, err := b.scmManager.CreatePullRequest(repo)
				if err != nil {
					return err
				}

				repo.SetPullRequest(int64(pullRequestID))

				log.Printf("repo '%s': created pull request and sleeping for %v", repo.Name, b.conf.General.Delay)

				b.sleep()
			}

			return nil
		})
	}

	errWait := group.Wait()

	if b.conf.General.Cleanup {
		defer b.clean()
	}

	// Only save repos to storage where a PR was created and Stateful or Auto Merge is set to true.
	if b.conf.General.Stateful || b.conf.SCM.PullRequest.AutoMerge {
		err = b.storageManager.Save(repos.GetSavable())
		if err != nil {
			return err
		}
	}

	return errWait
}

func (b *GoModBump) sleep() {
	time.Sleep(b.conf.General.Delay)
}

func (b *GoModBump) clean() {
	os.RemoveAll(b.conf.GetWorkDir())
}

func converge(workdir string, reposFromStorage repository.Repositories, reposFromSCM repository.Repositories) repository.Repositories {
	for n, repoFromSCM := range reposFromSCM {
		repoFromSCM.BaseDir = workdir

		for _, repoFromStorage := range reposFromStorage {
			repoFromStorage.BaseDir = workdir

			if repoFromSCM.ClonePath() == repoFromStorage.ClonePath() {
				reposFromSCM[n] = repoFromStorage
			}
		}
	}

	return reposFromSCM
}
