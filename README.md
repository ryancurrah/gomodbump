# gomodbump

Ensures your Go modules are using the latest minor and patch versions of your dependencies.

## Description

Go module bump will ensure your Go module repositories are using the latest dependencies. This is useful for organizations where they want to ensure their internal dependencies are always up to date.

## How It Works

Schedule `gomodbump` to run every `X` amount time in your favorite scheduler.

**NOTE:** It is recommended to setup a special user in your SCM just for updating Go modules so you can setup specific rules for ignoring that user when they push commits and only run CI on pull requests.

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

## Supported SCM

- [Bitbucket server](https://www.atlassian.com/software/bitbucket)

## Supported VCS

- [Git](https://git-scm.com/)

## Supported Storage

Storage is used to save the repository state, it contains the pull request information to use for the next execution.

- Local file

## TODO

- [ ] Implement batched pull request strategy
- [ ] Support S3 storage
- [ ] Support Github SCM

## Configuration

```yaml
general:
  cleanup: false                                   # Enabling this will prevent the work_dir from being cleaned up after running
  workers: 4                                       # Number of concurrent Go routines to process the repositories with
  work_dir: repos/                                 # Directory to clone the repositories to
  stateful: true                                   # Ensures you do not create more than 1 pull request for each repo. Requires storage to be configured

scm:
  pull_request:
    title: Updating go.mod dependencies
    description: Updating go.mod dependencies
    strategy: batch                                # Enabling the batch strategy will ensure we do not overwhelm CI by slowly updating modules (Not implemented yet)
    batch_size: 1                                  # Number of pull requests to open at a time
    batch_delay: 1m                                # Delay in seconds, minutes, hours between pull request batches
    auto_merge: true                               # Will automatically merge the pull request if it is mergeable. This enables stateful

  bitbucket_server:
    # BITBUCKET_SERVER_USERNAME env var required
    # BITBUCKET_SERVER_PASSWORD env var required
    url: http://127.0.0.1:7990/rest                # URL of the bitbucket server, must have /rest appended
    project_key: GO                                # Bitbucket project key to scan for repositories
    clone_type: http                               # Only http clone type support at the moment, later ssh may be supported

vcs:
  git:
    # GIT_USERNAME env var required
    # GIT_PASSWORD env var required
    source_branch: updating-go-modules             # Name of the source branch to create the current datetime is always appended
    target_branch: master                          # Name of the target branch to create the branch from and pull request against
    commit_message: Updating go.mod dependencies
    commit_author_name: FirstName LastName
    commit_author_email: admin@admin.com

bump:
  go_mod_tidy: true                                # Will run `go mod tidy` if set to true after each go get
  allowed_modules: []                              # List of allowed modules to update. If set any modules not in the allowed lists are blocked
  allowed_domains: []                              # List of allowed module domains to update. If set any modules not in the allowed lists are blocked
  blocked_modules: []                              # List of explicit modules to not update
  blocked_domains: []                              # List of explicit module domains to not update

storage:
  file:
    filename: gomodbump.json                       # Saves the state to the file specified here
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

## License

**MIT**
