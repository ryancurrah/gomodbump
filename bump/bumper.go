package bump

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/ryancurrah/gomodbump/repository"
)

var (
	goModFilename = "go.mod"
	goSumFilename = "go.sum"
)

// Configuration to use when bumping module versions
type Configuration struct {
	GoModTidy      bool     `yaml:"go_mod_tidy"`
	AllowedModules []string `yaml:"allowed_modules"`
	AllowedDomains []string `yaml:"allowed_domains"`
	BlockedModules []string `yaml:"blocked_modules"`
	BlockedDomains []string `yaml:"blocked_domains"`
}

// IsModuleAllowed returns true if the module is allowed to be updated
func (c Configuration) IsModuleAllowed(module string) bool {
	for _, allowedModule := range c.AllowedModules {
		if allowedModule == module {
			return true
		}
	}

	for _, allowedDomain := range c.AllowedDomains {
		if strings.HasPrefix(module, allowedDomain) {
			return true
		}
	}

	for _, blockedModule := range c.BlockedModules {
		if blockedModule == module {
			return false
		}
	}

	for _, blockedDomain := range c.BlockedDomains {
		if strings.HasPrefix(module, blockedDomain) {
			return false
		}
	}

	// If we have an allow list that means anything not in the allowed list is blocked
	if len(c.AllowedModules) > 0 || len(c.AllowedDomains) > 0 {
		return false
	}

	return true
}

// Bumper bumps all Go modules based on the settings provided
type Bumper struct {
	conf Configuration
}

// NewBumper initialises a new bumper
func NewBumper(conf Configuration) *Bumper {
	return &Bumper{conf: conf}
}

// Bump all the repositories Go module dependencies based on the configuration provided
func (b *Bumper) Bump(workers int, repos repository.Repositories) {
	reposToBump := make(chan *repository.Repository, len(repos))
	done := make(chan bool, len(repos))

	for worker := 1; worker <= workers; worker++ {
		go b.bump(reposToBump, done)
	}

	for n := range repos {
		reposToBump <- repos[n]
	}

	close(reposToBump)

	for range repos {
		<-done
	}
}

func (b *Bumper) bump(repos <-chan *repository.Repository, done chan<- bool) {
	for repo := range repos {
		err := isGoModule(repo.ClonePath())
		if err != nil {
			log.Printf("unable to bump '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		updates, err := getGoModuleUpdates(repo.ClonePath())
		if err != nil {
			log.Printf("unable to bump '%s' skipping it: %s", repo.Name, err)

			done <- true

			continue
		}

		filteredUpdates := make(repository.Updates, 0, len(updates))

		for n := range updates {
			if b.conf.IsModuleAllowed(updates[n].Module) {
				filteredUpdates = append(filteredUpdates, updates[n])
			}
		}

		if len(filteredUpdates) == 0 {
			log.Printf("repo %s has no updates, skipping", repo.Name)

			done <- true

			continue
		}

		log.Printf("repo %s has %d updates, running", repo.Name, len(filteredUpdates))

		for n := range filteredUpdates {
			err := updateGoModule(repo.ClonePath(), filteredUpdates[n].Module, *filteredUpdates[n].NewVersion)
			if err != nil {
				log.Printf("unable to update repo for %s skipping it: %s", repo.Name, err)

				done <- true

				continue
			}
		}

		err = runGoModTidy(repo.ClonePath())
		if err != nil {
			log.Printf("failed to run go mod tidy for repo %s: %s", repo.Name, err)

			done <- true

			continue
		}

		log.Printf("bumped go.mod for repo %s", repo.Name)

		repo.SetBumped(filteredUpdates)

		done <- true
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}

func isGoModule(workingDir string) error {
	goModFilePath := filepath.Join(workingDir, goModFilename)
	goSumFilePath := filepath.Join(workingDir, goSumFilename)

	if !fileExists(goModFilePath) {
		return fmt.Errorf("go.mod file does not exist in '%s'", goModFilePath)
	}

	if !fileExists(goSumFilePath) {
		return fmt.Errorf("go.sum file does not exist in '%s'", goModFilePath)
	}

	return nil
}

func getGoModuleUpdates(workingDir string) (repository.Updates, error) {
	template := "{{if (and (not (or .Main .Indirect)) .Update)}}{{.Path}}:{{.Version}}:{{.Update.Version}}{{end}}"

	cmd := exec.Command("go", "list", "-u", "-f", template, "-m", "all")
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ())

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to find updates for Go module: %s", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("unable to find updates for Go module: %s", err)
	}

	buf := new(bytes.Buffer)

	_, err = buf.ReadFrom(stdout)
	if err != nil {
		return nil, fmt.Errorf("unable to find updates for Go module: %s", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	updates := make(repository.Updates, 0, len(lines))

	for n := range lines {
		columns := strings.Split(lines[n], ":")

		if len(columns) != 3 { // nolint: gomnd
			continue
		}

		module := columns[0]
		if module == "" {
			continue
		}

		oldVersion, err := semver.NewVersion(columns[1])
		if err != nil {
			log.Printf("invalid old module semver in '%s' for module '%s': %s", workingDir, module, err)
			continue
		}

		newVersion, err := semver.NewVersion(columns[2])
		if err != nil {
			log.Printf("invalid new module semver in '%s' for module '%s': %s", workingDir, module, err)
			continue
		}

		updates = append(updates, &repository.Update{
			Module:     module,
			OldVersion: oldVersion,
			NewVersion: newVersion,
		})
	}

	return updates, nil
}

func updateGoModule(workingDir, module string, version semver.Version) error {
	moduleVersion := fmt.Sprintf("%s@v%s", module, version.String())

	cmd := exec.Command("go", "get", moduleVersion)
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ())

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to update module '%s': %s", module, err)
	}

	errMsg, _ := ioutil.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to update module '%s': %s %s", module, strings.TrimSpace(string(errMsg)), err)
	}

	return nil
}

func runGoModTidy(workingDir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ())

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	errMsg, _ := ioutil.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s: %s", strings.TrimSpace(string(errMsg)), err)
	}

	return nil
}
