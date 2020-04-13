package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ryancurrah/gomodbump/repository"
)

// FileStorage backend
type FileStorage struct {
	Filename string
}

// Save gomodbump repos to storage
func (s *FileStorage) Save(repos repository.Repositories) error {
	file, err := json.MarshalIndent(repos, "", "    ")
	if err != nil {
		return fmt.Errorf("unable to save to storage: %s", err)
	}

	err = ioutil.WriteFile(s.Filename, file, 0600)
	if err != nil {
		return fmt.Errorf("unable to save to storage: %s", err)
	}

	return nil
}

// Load gomodbump repos from storage
func (s *FileStorage) Load() (repository.Repositories, error) {
	if !fileExists(s.Filename) {
		return repository.Repositories{}, nil
	}

	file, err := ioutil.ReadFile(s.Filename)
	if err != nil {
		return nil, fmt.Errorf("unable to load from storage: %s", err)
	}

	repos := repository.Repositories{}

	err = json.Unmarshal(file, &repos)
	if err != nil {
		return nil, fmt.Errorf("unable to load from storage: %s", err)
	}

	return repos, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}
