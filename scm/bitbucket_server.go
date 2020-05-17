package scm

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	"github.com/mitchellh/mapstructure"
	"github.com/ryancurrah/gomodbump/repository"
)

// BitbucketServerConfig is the information required to interact with Bitbucket server
type BitbucketServerConfig struct {
	URL        string `yaml:"url"`
	Insecure   bool   `yaml:"insecure"`
	ProjectKey string `yaml:"project_key"`
	CloneType  string `yaml:"clone_type"`
	Username   string `yaml:"-"`
	Password   string `yaml:"-"`
	Token      string `yaml:"-"`
}

// BitbucketServer scm
type BitbucketServer struct {
	ctx         context.Context
	conf        BitbucketServerConfig
	pullRequest PullRequestConfig
	client      *bitbucketv1.APIClient
}

// NewBitbucketServer initializes a new Bitbucket SCM manager
func NewBitbucketServer(pullRequestConf PullRequestConfig, conf BitbucketServerConfig, cloneType string) *BitbucketServer {
	var ctx context.Context
	if strings.TrimSpace(conf.Token) == "" {
		ctx = context.WithValue(
			context.Background(),
			bitbucketv1.ContextBasicAuth, bitbucketv1.BasicAuth{UserName: conf.Username, Password: conf.Password},
		)
	} else {
		ctx = context.Background()
	}

	conf.CloneType = cloneType

	bitbucketServer := BitbucketServer{
		ctx:         ctx,
		pullRequest: pullRequestConf,
		conf:        conf,
	}

	bitbucketServer.client = bitbucketv1.NewAPIClient(
		bitbucketServer.ctx,
		bitbucketv1.NewConfiguration(conf.URL, func(config *bitbucketv1.Configuration) {
			if conf.Token != "" {
				config.AddDefaultHeader("Authorization", fmt.Sprintf("Bearer %s", conf.Token))
			}

			config.HTTPClient = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: conf.Insecure}, // nolint: gosec
				},
			}
		}),
	)

	return &bitbucketServer
}

// SCMType returns the SCM type
func (b *BitbucketServer) SCMType() repository.SCM {
	return repository.BitbucketServer
}

// GetRepositories that belong to the project
func (b *BitbucketServer) GetRepositories(vcsType repository.VCS) (repository.Repositories, error) {
	log.Printf("getting repos for bitbucket-server project %s", b.conf.ProjectKey)

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

// CreatePullRequest against the repos provided using the strategy provided
func (b *BitbucketServer) CreatePullRequest(repo *repository.Repository) (int, error) {
	return b.createPullRequest(repo)
}

// MergePullRequest merges all existing pull requests that can be merged
func (b *BitbucketServer) MergePullRequest(repo *repository.Repository) error {
	return b.mergePullRequest(repo)
}

func (b *BitbucketServer) createPullRequest(repo *repository.Repository) (int, error) {
	if repo.VCS != repository.Git {
		log.Print(vcsNotSupportedMsg(repo.VCS))

		return 0, nil
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
		return 0, fmt.Errorf("repo '%s': unable to create pull request: %s", repo.Name, err)
	}

	pullRequest, err := bitbucketv1.GetPullRequestResponse(response)
	if err != nil {
		return 0, fmt.Errorf("repo '%s': unable to create pull request: %s", repo.Name, err)
	}

	return pullRequest.ID, nil
}

func getBitbucketServerCloneURL(cloneType string, cloneLinks []bitbucketv1.CloneLink) string {
	for n := range cloneLinks {
		if strings.EqualFold(cloneLinks[n].Name, cloneType) {
			return cloneLinks[n].Href
		}
	}

	return ""
}

func (b *BitbucketServer) mergePullRequest(repo *repository.Repository) error {
	response, err := b.client.DefaultApi.GetPullRequest(repo.Parent, repo.Name, int(repo.PullRequestID))
	if err != nil {
		return fmt.Errorf("repo '%s': unable to merge pull request #%d: %s", repo.Name, repo.PullRequestID, err)
	}

	pullRequest, err := bitbucketv1.GetPullRequestResponse(response)
	if err != nil {
		return fmt.Errorf("repo '%s': unable to merge pull request #%d: %s", repo.Name, repo.PullRequestID, err)
	}

	if !pullRequest.Open {
		return nil
	}

	response, err = b.client.DefaultApi.CanMerge(repo.Parent, repo.Name, repo.PullRequestID)
	if err != nil {
		return fmt.Errorf("repo '%s': unable to get pull request #%d 'can merge' status: %s", repo.Name, repo.PullRequestID, err)
	}

	var merge bitbucketv1.MergeGetResponse

	err = mapstructure.Decode(response.Values, &merge)
	if err != nil {
		return fmt.Errorf("repo '%s': unable to get pull request #%d 'can merge' status: %s", repo.Name, repo.PullRequestID, err)
	}

	if !merge.CanMerge {
		log.Printf("repo '%s': unable to merge pull request #%d: %+v", repo.Name, repo.PullRequestID, merge.Vetoes)

		return nil
	}

	mergeMap := make(map[string]interface{})
	mergeMap["version"] = pullRequest.Version

	_, err = b.client.DefaultApi.Merge(repo.Parent, repo.Name, int(repo.PullRequestID), mergeMap, nil, []string{"application/json"})
	if err != nil {
		return fmt.Errorf("repo '%s': unable to merge pull request #%d: %s", repo.Name, repo.PullRequestID, err)
	}

	return nil
}

func vcsNotSupportedMsg(vcs repository.VCS) string {
	return fmt.Sprintf("scm '%s' does not support vcs type '%s': the following vcs types are supported [%s]", repository.BitbucketServer, vcs, repository.Git)
}
