ARG GOLANG_VERSION=1.14.2
ARG ALPINE_VERSION=3.11
ARG GOMODBUMP_VERSION=

# ---- Build container
FROM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION} AS builder
WORKDIR /gomodbump
COPY . .
RUN apk add --no-cache git
RUN go build -o gomodbump cmd/gomodbump/main.go

# ---- App container
FROM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION}
ENV BITBUCKET_SERVER_USERNAME=
ENV BITBUCKET_SERVER_PASSWORD=
ENV GIT_USERNAME=
ENV GIT_PASSWORD=
WORKDIR /
RUN apk --no-cache add ca-certificates
COPY --from=builder gomodbump/gomodbump /
ENTRYPOINT ./gomodbump
LABEL Name=gomodbump Version=${GOMODBUMP_VERSION}
