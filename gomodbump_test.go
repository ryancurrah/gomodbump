// nolint:scopelint
package gomodbump_test

import (
	"testing"

	"github.com/ryancurrah/gomodbump"
)

func TestConfigurationGetWorkDir(t *testing.T) {
	var tests = []struct {
		testName    string
		config      gomodbump.Configuration
		wantWorkDir string
	}{
		{
			"should return a cleaned workdir path",
			gomodbump.Configuration{General: gomodbump.GeneralConfig{WorkDir: "repos/"}},
			"repos",
		},
		{
			"should return a cleaned workdir path when it is dirty",
			gomodbump.Configuration{General: gomodbump.GeneralConfig{WorkDir: "./repos//"}},
			"repos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			workDir := tt.config.GetWorkDir()
			if workDir != tt.wantWorkDir {
				t.Errorf("got '%v' want '%v'", workDir, tt.wantWorkDir)
			}
		})
	}
}
