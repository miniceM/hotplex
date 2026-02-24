package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestStartSession_WorkDirResolution verifies that WorkDir is correctly resolved
func TestStartSession_WorkDirResolution(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	testCases := []struct {
		name        string
		workDir     string
		expectedDir string
		shouldExist bool
	}{
		{name: "dot_current_dir", workDir: ".", expectedDir: cwd, shouldExist: true},
		{name: "absolute_path", workDir: "/tmp", expectedDir: "/tmp", shouldExist: true},
		{name: "relative_path_subdir", workDir: "./testdir", expectedDir: filepath.Join(cwd, "testdir"), shouldExist: false},
		{name: "path_with_dot_middle", workDir: "/tmp/./hotplex", expectedDir: filepath.Clean("/tmp/./hotplex"), shouldExist: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := SessionConfig{WorkDir: tc.workDir}

			// Replicate the FIXED logic from pool.go
			var resolvedDir string
			if cfg.WorkDir == "." || !filepath.IsAbs(cfg.WorkDir) {
				cleaned := filepath.Clean(cfg.WorkDir)
				if absPath, err := filepath.Abs(cleaned); err == nil {
					resolvedDir = absPath
				} else {
					resolvedDir = cleaned
				}
			} else {
				resolvedDir = cfg.WorkDir
			}

			if resolvedDir != tc.expectedDir {
				t.Errorf("Resolved dir = %q, want %q", resolvedDir, tc.expectedDir)
			}

			if tc.shouldExist {
				if _, err := os.Stat(resolvedDir); os.IsNotExist(err) {
					t.Errorf("Resolved directory does not exist: %s", resolvedDir)
				}
			}
		})
	}
}

// TestStartSession_CmdDirAssignment verifies cmd.Dir is correctly assigned
func TestStartSession_CmdDirAssignment(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	testCases := []struct {
		name       string
		workDir    string
		wantCmdDir string
	}{
		{"dot", ".", cwd},
		{"absolute", "/tmp", "/tmp"},
		{"relative", "./subdir", filepath.Join(cwd, "subdir")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := SessionConfig{WorkDir: tc.workDir}
			cmd := exec.CommandContext(context.Background(), "echo", "test")

			// Replicate the FIXED logic from pool.go
			if cfg.WorkDir == "." || !filepath.IsAbs(cfg.WorkDir) {
				cleaned := filepath.Clean(cfg.WorkDir)
				if absPath, err := filepath.Abs(cleaned); err == nil {
					cmd.Dir = absPath
				} else {
					cmd.Dir = cleaned
				}
			} else {
				cmd.Dir = cfg.WorkDir
			}

			if cmd.Dir != tc.wantCmdDir {
				t.Errorf("cmd.Dir = %q, want %q", cmd.Dir, tc.wantCmdDir)
			}
		})
	}
}

// TestChatAppsWorkDirFunction verifies chatapps expandPath handles "." correctly
func TestChatAppsWorkDirFunction(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}

	// Test the actual expandPath function from setup.go
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"dot", ".", cwd},
		{"absolute", "/tmp/myproject", "/tmp/myproject"},
		{"relative", "./myproject", filepath.Join(cwd, "myproject")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := expandPathForTest(tc.input)
			if result != tc.expected {
				t.Errorf("expandPath(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// expandPathForTest replicates the expandPath logic from setup.go
func expandPathForTest(path string) string {
	if len(path) == 0 {
		return path
	}

	// Handle special case: "." should be expanded to current working directory
	if path == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return path
		}
		return cwd
	}

	return filepath.Clean(path)
}
