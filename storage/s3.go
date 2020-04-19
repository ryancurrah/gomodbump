package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/ryancurrah/gomodbump/repository"
)

// S3StorageConfig configuration of AWS S3 file storage
type S3StorageConfig struct {
	Region     string `yaml:"region"`
	Bucketname string `yaml:"bucketname"`
	Filename   string `yam:"filename"`
}

// S3Storage backend
type S3Storage struct {
	conf   S3StorageConfig
	client *s3.S3
}

// NewS3Storage initializes a new S3 storage backend
func NewS3Storage(conf S3StorageConfig) (*S3Storage, error) {
	if conf.Filename == "" {
		conf.Filename = defaultFilename
	}

	sess, err := session.NewSession(&aws.Config{Region: aws.String(conf.Region)})
	if err != nil {
		return nil, err
	}

	return &S3Storage{
		conf:   conf,
		client: s3.New(sess),
	}, nil
}

// Save gomodbump repos to storage
func (s *S3Storage) Save(repos repository.Repositories) error {
	file, err := json.MarshalIndent(repos, "", "    ")
	if err != nil {
		return fmt.Errorf("unable to save to storage: %s", err)
	}

	input := &s3.PutObjectInput{
		Body:   aws.ReadSeekCloser(bytes.NewReader(file)),
		Bucket: aws.String(s.conf.Bucketname),
		Key:    aws.String(s.conf.Filename),
	}

	_, err = s.client.PutObject(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			return aerr
		}

		return err
	}

	return nil
}

// Load gomodbump repos from storage
func (s *S3Storage) Load() (repository.Repositories, error) {
	repos := repository.Repositories{}

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.conf.Bucketname),
		Key:    aws.String(s.conf.Filename),
	}

	object, err := s.client.GetObject(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				return repos, nil
			default:
				return nil, fmt.Errorf("unable to load from storage: %s: %s", s.conf.Bucketname, aerr)
			}
		}

		return nil, fmt.Errorf("unable to load from storage: %s", err)
	}

	file, err := ioutil.ReadAll(object.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to load from storage: %s", err)
	}

	err = json.Unmarshal(file, &repos)
	if err != nil {
		return nil, fmt.Errorf("unable to load from storage: %s", err)
	}

	return repos, nil
}
