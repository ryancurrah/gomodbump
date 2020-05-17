package vcs

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/gookit/color"
	"github.com/ryancurrah/gomodbump/repository"
	"golang.org/x/crypto/ssh"
)

var (
	goModFilename = "go.mod"
	goSumFilename = "go.sum"
)

// GitConfig are the options to use for Git VCS.
type GitConfig struct {
	SourceBranch      string `yaml:"source_branch"`
	TargetBranch      string `yaml:"target_branch"`
	CommitMessage     string `yaml:"commit_message"`
	CommitAuthorName  string `yaml:"commit_author_name"`
	CommitAuthorEmail string `yaml:"commit_author_email"`
	Insecure          bool   `yaml:"insecure"`
	Username          string `yaml:"-"`
	Password          string `yaml:"-"`
	Token             string `yaml:"-"`
}

// Git is a version control system supported by gomodbump.
type Git struct {
	conf        GitConfig
	auth        transport.AuthMethod
	colorWriter ColorWriter
}

// NewGit initializes a new VCS manager.
func NewGit(conf GitConfig, authType string) (*Git, error) {
	switch authType {
	case "ssh":
		auth, err := gitssh.NewSSHAgentAuth("git")
		if err != nil {
			return nil, err
		}

		if conf.Insecure {
			auth.HostKeyCallback = ssh.InsecureIgnoreHostKey() // nolint: gosec
		}

		return &Git{
			conf: conf,
			auth: auth,
		}, nil
	default:
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: conf.Insecure}, // nolint: gosec
			},
		}

		client.InstallProtocol("https", githttp.NewClient(httpClient))

		if strings.TrimSpace(conf.Token) != "" {
			return &Git{
				conf: conf,
				auth: &githttp.TokenAuth{Token: conf.Token},
			}, nil
		}

		return &Git{
			conf: conf,
			auth: &githttp.BasicAuth{
				Username: conf.Username,
				Password: conf.Password,
			},
			colorWriter: ColorWriter{Color: color.LightBlue},
		}, nil
	}
}

// GetSourceBranch returns the source branch to use for creating changes.
func (g *Git) GetSourceBranch() string {
	return fmt.Sprintf("%s-%s", g.conf.SourceBranch, time.Now().Format("20060102150405"))
}

// GetTargetBranch returns the branch the source branch was checked out from.
func (g *Git) GetTargetBranch() string {
	return g.conf.TargetBranch
}

// VCSType returns the VCS type.
func (g *Git) VCSType() repository.VCS {
	return repository.Git
}

// Clone all the repos provided and return the ones that successfully cloned.
func (g *Git) Clone(repo *repository.Repository) (*git.Repository, error) {
	return g.clone(repo)
}

// Push changed files.
func (g *Git) Push(repo *repository.Repository) error {
	return g.push(repo)
}

// DeleteBranch from remote.
func (g *Git) DeleteBranch(repo *repository.Repository) error {
	return g.deleteBranch(repo)
}

func (g *Git) clone(repo *repository.Repository) (*git.Repository, error) {
	if repo.SourceBranch == "" {
		repo.SourceBranch = g.GetSourceBranch()
	}

	if repo.TargetBranch == "" {
		repo.TargetBranch = g.GetTargetBranch()
	}

	cloneOpts := git.CloneOptions{
		URL:          repo.URL,
		SingleBranch: true,
		Auth:         g.auth,
		Progress:     g.colorWriter,
	}

	gitRepo, err := git.PlainClone(repo.ClonePath(), false, &cloneOpts)
	if err != nil {
		return nil, fmt.Errorf("repo '%s': unable to git clone, skipping: %s", repo.Name, err)
	}

	worktree, err := gitRepo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("repo '%s': unable to git clone, skipping: %s", repo.Name, err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(repo.SourceBranch),
		Create: true,
	})
	if err != nil {
		return nil, fmt.Errorf("repo '%s': unable to git clone, skipping: %s", repo.Name, err)
	}

	log.Printf("repo '%s': was cloned successfully", repo.Name)

	return gitRepo, nil
}

func (g *Git) push(repo *repository.Repository) error {
	log.Printf("repo '%s': pushing commits to remote", repo.Name)

	worktree, err := repo.GitRepo.Worktree()
	if err != nil {
		return fmt.Errorf("repo '%s': unable to push, skipping: %s", repo.Name, err)
	}

	_, err = worktree.Add(goModFilename)
	if err != nil {
		return fmt.Errorf("repo '%s': unable to push, skipping: %s", repo.Name, err)
	}

	_, err = worktree.Add(goSumFilename)
	if err != nil {
		return fmt.Errorf("repo '%s': unable to push, skipping: %s", repo.Name, err)
	}

	_, err = worktree.Commit(g.conf.CommitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  g.conf.CommitAuthorName,
			Email: g.conf.CommitAuthorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("repo '%s': unable to push, skipping: %s", repo.Name, err)
	}

	err = repo.GitRepo.Push(&git.PushOptions{Auth: g.auth, Progress: g.colorWriter})
	if err != nil {
		return fmt.Errorf("repo '%s': unable to push, skipping: %s", repo.Name, err)
	}

	return nil
}

func (g *Git) deleteBranch(repo *repository.Repository) error {
	var branchExistsInRemote bool

	remote, err := repo.GitRepo.Remote("origin")
	if err != nil {
		return fmt.Errorf("repo '%s': unable to delete branch %s: %w", repo.Name, repo.SourceBranch, err)
	}

	remoteRefs, err := remote.List(&git.ListOptions{Auth: g.auth})
	if err != nil {
		return fmt.Errorf("repo '%s': fetching unable to delete branch %s: %w", repo.Name, repo.SourceBranch, err)
	}

	for n := range remoteRefs {
		remoteBranchName := remoteRefs[n].Name().Short()

		if remoteBranchName == repo.SourceBranch {
			branchExistsInRemote = true
		}
	}

	if !branchExistsInRemote {
		return nil
	}

	err = repo.GitRepo.Push(&git.PushOptions{
		Auth:     g.auth,
		RefSpecs: []config.RefSpec{config.RefSpec(fmt.Sprintf(":refs/heads/%s", repo.SourceBranch))},
		Progress: g.colorWriter,
	})
	if err != nil {
		return fmt.Errorf("repo '%s': unable to delete branch %s: %w", repo.Name, repo.SourceBranch, err)
	}

	log.Printf("repo '%s': branch %s cleaned up successfully", repo.Name, repo.SourceBranch)

	return nil
}

// ColorWriter writes output to stdout using the chosen color.
type ColorWriter struct {
	Color color.Color
}

func (c ColorWriter) Write(p []byte) (int, error) {
	c.Color.Print(string(p))
	return len(p), nil
}
