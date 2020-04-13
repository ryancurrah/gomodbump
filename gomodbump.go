package gomodbump

import (
	"os"
	"path/filepath"

	"github.com/ryancurrah/gomodbump/bump"
	"github.com/ryancurrah/gomodbump/repository"
	"github.com/ryancurrah/gomodbump/scm"
	"github.com/ryancurrah/gomodbump/storage"
	"github.com/ryancurrah/gomodbump/vcs"
)

type scmManager interface {
	SCMType() repository.SCM
	GetRepositories(vcsType repository.VCS) (repository.Repositories, error)
	MergePullRequest(workers int, repos repository.Repositories)
	CreatePullRequest(workers int, repos repository.Repositories)
}

type vcsManager interface {
	GetSourceBranch() string
	GetTargetBranch() string
	VCSType() repository.VCS
	Clone(workers int, repos repository.Repositories)
	Push(workers int, repos repository.Repositories)
}

type bumper interface {
	Bump(workers int, repos repository.Repositories)
}

type storageManager interface {
	Save(repos repository.Repositories) error
	Load() (repository.Repositories, error)
}

// GeneralConfig are general settings for this package
type GeneralConfig struct {
	Cleanup bool   `yaml:"cleanup"`
	Workers int    `yaml:"workers"`
	WorkDir string `yaml:"work_dir"`
}

// SourceCodeManagementConfig used to create pull requests and get repos
type SourceCodeManagementConfig struct {
	PullRequest     scm.PullRequestConfig     `yaml:"pull_request"`
	BitbucketServer scm.BitbucketServerConfig `yaml:"bitbucket_server"`
}

// VersionControlSystemConfig used to work with the repos
type VersionControlSystemConfig struct {
	Git vcs.GitConfig `yaml:"git"`
}

// FileStorageConfig configuration of file storage
type FileStorageConfig struct {
	Filename string `yam:"filename"`
}

// StorageConfig allow different file storage backends
type StorageConfig struct {
	File FileStorageConfig `yam:"file"`
}

// Configuration for Go mod bump
type Configuration struct {
	General GeneralConfig              `yaml:"general"`
	SCM     SourceCodeManagementConfig `yaml:"scm"`
	VCS     VersionControlSystemConfig `yaml:"vcs"`
	Bump    bump.Configuration         `yaml:"bump"`
	Storage StorageConfig              `yaml:"storage"`
}

// GetWorkDir returns the working dir path cleaned
func (c Configuration) GetWorkDir() string {
	return filepath.Clean(c.General.WorkDir)
}

// GoModBump does the bumping of versions
type GoModBump struct {
	conf           Configuration
	scmManager     scmManager
	vcsManager     vcsManager
	bumper         bumper
	storageManager storageManager
}

// NewGoModBump initializes a Go Mod Bump struct
func NewGoModBump(conf Configuration) GoModBump {
	return GoModBump{
		conf:       conf,
		scmManager: scm.NewBitbucketServer(conf.SCM.PullRequest, conf.SCM.BitbucketServer),
		vcsManager: vcs.NewGit(conf.VCS.Git),
		bumper:     bump.NewBumper(conf.Bump),
		storageManager: &storage.FileStorage{
			Filename: conf.Storage.File.Filename,
		},
	}
}

// Run Go Mod Bump
func (b *GoModBump) Run() error {
	// Cleanup working directory before running
	b.clean()

	// Get the repos from the last the run, this contains PR infor
	reposFromStorage, err := b.storageManager.Load()
	if err != nil {
		return err
	}

	// Get current repos from SCM
	reposFromSCM, err := b.scmManager.GetRepositories(b.vcsManager.VCSType())
	if err != nil {
		return err
	}

	// Converge the repos from storage into the repos from SCM
	repos := converge(b.conf.GetWorkDir(), reposFromStorage, reposFromSCM)

	// If any of the repos have a pull request open and they are mergeable, merge them (If auto_merge=true)
	b.scmManager.MergePullRequest(b.conf.General.Workers, repos.GetMergeable(b.scmManager.SCMType()))

	// Clone repos locally that are cloneable
	b.vcsManager.Clone(b.conf.General.Workers, repos.GetCloneable(b.vcsManager.VCSType()))

	// Bump repos Go modules locally that are bumpable
	b.bumper.Bump(b.conf.General.Workers, repos.GetBumpable())

	// Push repos locally that are pushable
	b.vcsManager.Push(b.conf.General.Workers, repos.GetPushable(b.vcsManager.VCSType()))

	// Create pull requests for repos where they are PRable
	b.scmManager.CreatePullRequest(b.conf.General.Workers, repos.GetPRable(b.scmManager.SCMType()))

	if b.conf.General.Cleanup {
		defer b.clean()
	}

	// Only save repos to storage where a PR was created and Auto Merge is set to true
	// In order to support auto merge we need to save repo state
	if b.conf.SCM.PullRequest.AutoMerge {
		err = b.storageManager.Save(repos.GetSavable())
		if err != nil {
			return err
		}
	}

	return nil
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
