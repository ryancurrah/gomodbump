current_dir = $(shell pwd)
version = $(shell printf '%s' $$(cat VERSION))

.PHONEY: start-bitbucket-server
start-bitbucket-server:
	docker run -v bitbucketVolume:/var/atlassian/application-data/bitbucket --name="bitbucket" -d -p 7990:7990 -p 7999:7999 atlassian/bitbucket-server

.PHONEY: lint
lint:
	golangci-lint run -v --enable-all --disable funlen,gochecknoglobals,lll ./...

.PHONEY: build
build:
	go build -o gomodbump cmd/gomodbump/main.go

.PHONEY: test
test:
	go test -v -coverprofile coverage.out 

.PHONEY: cover
cover:
	gocover-cobertura < coverage.out > coverage.xml

.PHONEY: dockerbuild
dockerbuild:
	docker build --build-arg GOMODBUMP_VERSION=${version} --tag ryancurrah/gomodbump:${version} .
 
.PHONEY: run-basicauth
run-basicauth: build
	GIT_USERNAME=admin GIT_PASSWORD=admin BITBUCKET_SERVER_USERNAME=admin BITBUCKET_SERVER_PASSWORD=admin ./gomodbump

.PHONEY: run-tokenauth
run-tokenauth: build
	GIT_TOKEN="OTEzNDY5MDAyMjE4OthQjKPe0x2+PKEOex2f/sd1o53K" BITBUCKET_SERVER_TOKEN="OTEzNDY5MDAyMjE4OthQjKPe0x2+PKEOex2f/sd1o53K" ./gomodbump

.PHONEY: run-sshauth
run-sshauth: build
	BITBUCKET_SERVER_TOKEN="OTEzNDY5MDAyMjE4OthQjKPe0x2+PKEOex2f/sd1o53K" ./gomodbump

.PHONEY: dockerrun
dockerrun: dockerbuild
	docker run --network host -e GIT_USERNAME=admin -e GIT_PASSWORD=admin -e BITBUCKET_SERVER_USERNAME=admin -e BITBUCKET_SERVER_PASSWORD=admin -v "${current_dir}/.gomodbump.yaml:/.gomodbump.yaml" ryancurrah/gomodbump:latest

.PHONEY: release
release:
	git tag ${version}
	git push --tags
	goreleaser --skip-validate --rm-dist

.PHONEY: clean
clean:
	rm -rf repos/ dist/
	rm -f gomodbump.json gomodbump

.PHONEY: install-tools-mac
install-tools-mac:
	pip3 install bumpversion
	brew install goreleaser/tap/goreleaser

.PHONEY: install-go-tools
install-go-tools:
	go get github.com/t-yuki/gocover-cobertura
