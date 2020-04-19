# gomodbump

<img src="https://storage.googleapis.com/gopherizeme.appspot.com/gophers/68a9f253a5c70bd1fb39e8ac9c07cb2c434f5952.png" width="30%">

Ensures your Go modules are using the latest minor and patch versions of your dependencies.

Similar to https://dependabot.com/ or https://renovatebot.com/ but focused solely on Go.

## Description

Go module bump will ensure your Go module repositories are using the latest dependencies. This is useful for organizations where they want to ensure their internal dependencies are always up to date.

## How It Works

Schedule `gomodbump` to run every `X` amount time in your favorite scheduler.

1. Gets repositories from storage (If the file exists)
2. Gets repositories from the SCM server
3. Merges any existing pull requests for a repository that is mergeable
4. If `stateful` or `auto_merge` is `true` and a pull request is already open for the repository it will not be processed any further
5. Clones repositories to local disk
6. If a repository is not a Go module it will not be processed any further
7. Bumps any updatable dependency versions that are allowed or not blocked. By default all dependencies are allowed. If `go_mod_tidy` is `true` that will be run after updating
8. Pushes updates to the `go.mod` and `go.sum` files to SCM server for each repository
9. Creates pull requests for all pushed repositories in the SCM server
10. Saves the state to the specified storage backend

## Supported GO Environment Variables

- `GOPROXY`
- `GOPRIVATE`
- `GOSUMDB`
- `GONOSUMDB`

## Supported SCM

- [Bitbucket server](https://www.atlassian.com/software/bitbucket)

## Supported VCS

- [Git](https://git-scm.com/)

## Supported Storage

Storage is used to save the repository state, it contains the pull request information to use for the next execution.

- Local file
- AWS S3

## Supported Auth

- Basic (bitbucket-server, git)
- Token (bitbucket-server, git)
- ssh-agent (git)

## TODO

- [ ] Support Github SCM

## Configuration

```yaml
general:
  cleanup: true                                    # Enabling this will prevent the work_dir from being cleaned up after running
  workers: 5                                       # Number of concurrent Go routines to process the repositories with
  work_dir: repos/                                 # Directory to clone the repositories to
  stateful: true                                   # Ensures you do not create more than 1 pull request for each repo. Requires storage to be configured
  clone_type: http                                 # http or ssh

scm:
  pull_request:
    title: Updating go.mod dependencies
    description: Updating go.mod dependencies
    delay: 10s                                     # Delay in seconds or minutes or hours after creating a pull request in order to not overwhelm your CI
    auto_merge: true                               # Will automatically merge the pull request if it is mergeable. This enables stateful

  bitbucket_server:
    # BITBUCKET_SERVER_USERNAME env var required
    # BITBUCKET_SERVER_PASSWORD env var required
    # OR
    # BITBUCKET_TOKEN env var required
    url: http://127.0.0.1:7990/rest                # URL of the bitbucket server, must have /rest appended
    insecure: false                                # Disable SSL verification for self-signed certs or internal CAs
    project_key: GO                                # Bitbucket project key to scan for repositories

vcs:
  git:
    # GIT_USERNAME env var required for basic auth
    # GIT_PASSWORD env var required for basic auth
    # OR
    # GIT_TOKEN env var required for token auth
    # OR
    # If clone_type ssh is specified will use ssh-agent. Ensure a host key is in your known hosts file or else enable insecure
    insecure: false                                # Disable SSL verification for self-signed certs or internal CAs OR disables host key checking for SSH
    source_branch: updating-go-modules             # Name of the source branch to create the current date time is always appended
    target_branch: master                          # Name of the target branch to create the branch from and pull request against
    commit_message: Updating go.mod dependencies
    commit_author_name: FirstName LastName
    commit_author_email: admin@admin.com
    delay: 10s                                     # Delay in seconds or minutes or hours after pushing to the remote in order to not overwhelm your CI

bump:
  go_mod_tidy: true                                # Will run `go mod tidy` if set to true after updating a repository
  allowed_modules: []                              # List of allowed modules to update. If set any modules not in the allowed lists are blocked
  allowed_domains: []                              # List of allowed module domains to update. If set any modules not in the allowed lists are blocked
  blocked_modules: []                              # List of explicit modules to not update
  blocked_domains: []                              # List of explicit module domains to not update

storage:
  file:
    filename: gomodbump.json                       # Saves the state to the file specified here

  # s3:
  #   # AWS_ACCESS_KEY_ID env var required
  #   # AWS_SECRET_ACCESS_KEY env var required
  #   # OR
  #   # ~/.aws/credentials
  #   region: "us-east-1"                            # Region to use for the S3 client
  #   bucketname: "gomodbump"                        # Name of the S3 bucket to get and put the state file
  #   filename: gomodbump.json                       # Saves the state to the file specified here
```

## Example

Running the binary:

```
GIT_USERNAME=admin \
  GIT_PASSWORD=admin \
  BITBUCKET_SERVER_USERNAME=admin \
  BITBUCKET_SERVER_PASSWORD=admin \
  ./gomodbump
```

Running the Docker image:

```
docker run \
  -e GIT_USERNAME=admin \
  -e GIT_PASSWORD=admin \
  -e BITBUCKET_SERVER_USERNAME=admin \
  -e BITBUCKET_SERVER_PASSWORD=admin \
  -v "$(pwd)/.gomodbump.yaml:/.gomodbump.yaml" \
  -v "gomodbumpVolume:/storage"
  ryancurrah/gomodbump:latest
```

## Install

```
go get -u github.com/ryancurrah/gomodbump/cmd/gomodbump
```

## Go proxy caveats

The `go list` command is used to determine if the module has an update. If the Go proxy environment variable is set and the proxy is not reachable it will silently fail. So ensure your proxy works manually. Any suggestions on solving this issue would be greatly appreciated.

## License

**MIT**
