current_dir = $(shell pwd)

.PHONEY: start-bitbucket-server
start-bitbucket-server:
	docker run -v bitbucketVolume:/var/atlassian/application-data/bitbucket --name="bitbucket" -d -p 7990:7990 -p 7999:7999 atlassian/bitbucket-server

.PHONEY: lint
lint:
	golangci-lint run -v --enable-all --disable funlen,gochecknoglobals,lll ./...

.PHONEY: build
build:
	go build -o gomodbump cmd/gomodbump/main.go

.PHONEY: dockerbuild
dockerbuild:
	docker build --arg GOMODBUMP_VERSION=$(shell cat VERSION) . --tag ryancurrah/gomodbump:latest

.PHONEY: run
run: build
	GIT_USERNAME=admin GIT_PASSWORD=admin BITBUCKET_SERVER_USERNAME=admin BITBUCKET_SERVER_PASSWORD=admin ./gomodbump

.PHONEY: dockerrun
dockerrun: dockerbuild
	docker run --network host -e GIT_USERNAME=admin -e GIT_PASSWORD=admin -e BITBUCKET_SERVER_USERNAME=admin -e BITBUCKET_SERVER_PASSWORD=admin -v "${current_dir}/.gomodbump.yaml:/.gomodbump.yaml" ryancurrah/gomodbump:latest

.PHONEY: release
release:
	bumpversion --current-version $(shell cat VERSION) minor VERSION
	git add VERSION
	git commit -m "Released version $(shell cat VERSION)"
	git tag $(shell cat VERSION)
	git push --tags
	goreleaser --rm-dist

.PHONEY: clean
clean:
	rm -rf repos/
	rm -f gomodbump.json gomodbump

.PHONEY: install-tools-mac
install-tools-mac:
	pip3 install bumpversion
	brew install goreleaser/tap/goreleaser
