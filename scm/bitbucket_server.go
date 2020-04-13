package scm

import (
	"context"
	"fmt"
	"log"
	"strings"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/mitchellh/mapstructure"
	"github.com/ryancurrah/gomodbump/repository"
)

// BitbucketServerConfig is the information required to interact with Bitbucket server
type BitbucketServerConfig struct {
	URL        string `yaml:"url"`
	ProjectKey string `yaml:"project_key"`
	CloneType  string `yaml:"clone_type"`
	Username   string `yaml:"-"`
	Password   string `yaml:"-"`
}

// BitbucketServer scm
type BitbucketServer struct {
	ctx         context.Context
	conf        BitbucketServerConfig
	pullRequest PullRequestConfig
	client      *bitbucketv1.APIClient
}

// NewBitbucketServer initializes a new Bitbucket SCM manager
func NewBitbucketServer(pullRequestConf PullRequestConfig, conf BitbucketServerConfig) *BitbucketServer {
	bitbucketServer := BitbucketServer{
		ctx: context.WithValue(
			context.Background(),
			bitbucketv1.ContextBasicAuth, bitbucketv1.BasicAuth{UserName: conf.Username, Password: conf.Password},
		),
		pullRequest: pullRequestConf,
		conf:        conf,
	}
	bitbucketServer.client = bitbucketv1.NewAPIClient(
		bitbucketServer.ctx,
		bitbucketv1.NewConfiguration(conf.URL),
	)

	return &bitbucketServer
}

// SCMType returns the SCM type
func (b *BitbucketServer) SCMType() repository.SCM {
	return repository.BitbucketServer
}

// GetRepositories that belong to the project
func (b *BitbucketServer) GetRepositories(vcsType repository.VCS) (repository.Repositories, error) {
	if vcsType != repository.Git {
		return nil, fmt.Errorf(vcsNotSupportedMsg(vcsType))
	}

	response, err := b.client.DefaultApi.GetRepositories(b.conf.ProjectKey)
	if err != nil {
		return nil, err
	}

	bitbucketRepos, err := bitbucketv1.GetRepositoriesResponse(response)
	if err != nil {
		return nil, err
	}

	repos := make(repository.Repositories, len(bitbucketRepos))
	for n := range bitbucketRepos {
		repos[n] = repository.NewRepository(
			bitbucketRepos[n].Slug,
			getBitbucketServerCloneURL(b.conf.CloneType, bitbucketRepos[n].Links.Clone),
			b.conf.ProjectKey,
			repository.BitbucketServer,
			vcsType,
		)
	}

	return repos, nil
}

// MergePullRequest merges all existing pull requests that can be merged
func (b *BitbucketServer) MergePullRequest(workers int, repos repository.Repositories) {
	reposToMerge := make(chan *repository.Repository, len(repos))
	done := make(chan bool, len(repos))

	for worker := 1; worker <= workers; worker++ {
		go b.mergePullRequest(reposToMerge, done)
	}

	for n := range repos {
		reposToMerge <- repos[n]
	}

	close(reposToMerge)

	for range repos {
		<-done
	}
}

// CreatePullRequest against the repos provided using the strategy provided
func (b *BitbucketServer) CreatePullRequest(workers int, repos repository.Repositories) {
	reposToPR := make(chan *repository.Repository, len(repos))
	done := make(chan bool, len(repos))

	for worker := 1; worker <= workers; worker++ {
		go b.createPullRequest(reposToPR, done)
	}

	for n := range repos {
		reposToPR <- repos[n]
	}

	close(reposToPR)

	for range repos {
		<-done
	}
}

func (b *BitbucketServer) createPullRequest(repos <-chan *repository.Repository, done chan<- bool) {
	for repo := range repos {
		if repo.VCS != repository.Git {
			log.Printf(vcsNotSupportedMsg(repo.VCS))
		}

		response, err := b.client.DefaultApi.CreatePullRequest(b.conf.ProjectKey, repo.Name, bitbucketv1.PullRequest{
			Title:       b.pullRequest.Title,
			Description: b.pullRequest.Description,
			FromRef: bitbucketv1.PullRequestRef{
				ID: fmt.Sprintf("refs/heads/%s", repo.SourceBranch),
				Repository: bitbucketv1.Repository{
					Slug: repo.Name,
					Project: &bitbucketv1.Project{
						Key: b.conf.ProjectKey,
					},
				},
			},
			ToRef: bitbucketv1.PullRequestRef{
				ID: fmt.Sprintf("refs/heads/%s", repo.TargetBranch),
				Repository: bitbucketv1.Repository{
					Slug: repo.Name,
					Project: &bitbucketv1.Project{
						Key: b.conf.ProjectKey,
					},
				},
			},
		})
		if err != nil {
			log.Printf("unable to create pull request for repo '%s': %s", repo.Name, err)

			done <- true

			continue
		}

		pullRequest, err := bitbucketv1.GetPullRequestResponse(response)
		if err != nil {
			log.Printf("unable to create pull request for repo '%s': %s", repo.Name, err)

			done <- true

			continue
		}

		repo.SetPullRequest(int64(pullRequest.ID))
		done <- true
	}
}

func getBitbucketServerCloneURL(cloneType string, cloneLinks []bitbucketv1.CloneLink) string {
	for n := range cloneLinks {
		if strings.EqualFold(cloneLinks[n].Name, cloneType) {
			return cloneLinks[n].Href
		}
	}

	return ""
}

func (b *BitbucketServer) mergePullRequest(repos <-chan *repository.Repository, done chan<- bool) {
	for repo := range repos {
		response, err := b.client.DefaultApi.GetPullRequest(repo.Parent, repo.Name, int(repo.PullRequestID))
		if err != nil {
			log.Printf("unable to merge pull request #%d for repo '%s': %s", repo.PullRequestID, repo.Name, err)

			done <- true

			continue
		}

		pullRequest, err := bitbucketv1.GetPullRequestResponse(response)
		if err != nil {
			log.Printf("unable to merge pull request #%d for repo '%s': %s", repo.PullRequestID, repo.Name, err)

			done <- true

			continue
		}

		if !pullRequest.Open {
			repo.ResetState()

			done <- true

			continue
		}

		response, err = b.client.DefaultApi.CanMerge(repo.Parent, repo.Name, repo.PullRequestID)
		if err != nil {
			log.Printf("unable to get pull request #%d 'can merge' status for repo '%s': %s", repo.PullRequestID, repo.Name, err)

			done <- true

			continue
		}

		var merge bitbucketv1.MergeGetResponse
		err = mapstructure.Decode(response.Values, &merge)
		if err != nil {
			log.Printf("unable to get pull request #%d 'can merge' status for repo '%s': %s", repo.PullRequestID, repo.Name, err)

			done <- true

			continue
		}

		if !merge.CanMerge {
			log.Printf("unable to merge pull request #%d for repo '%s': %+v", repo.PullRequestID, repo.Name, merge.Vetoes)

			done <- true

			continue
		}

		mergeMap := make(map[string]interface{})
		mergeMap["version"] = pullRequest.Version

		response, err = b.client.DefaultApi.Merge(repo.Parent, repo.Name, int(repo.PullRequestID), mergeMap, nil, []string{"application/json"})
		if err != nil {
			log.Printf("unable to merge pull request #%d for repo '%s': %s", repo.PullRequestID, repo.Name, err)

			done <- true

			continue
		}

		repo.ResetState()
		done <- true
	}
}

func vcsNotSupportedMsg(vcs repository.VCS) string {
	return fmt.Sprintf("scm '%s' does not support vcs type '%s': The following vcs types are supported [%s]", repository.BitbucketServer, vcs, repository.Git)
}
