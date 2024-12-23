package executor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"
)

type TestResult struct {
	Success bool
	Output  string
	Error   error
}

type TestRunner struct {
	workDir string
}

func NewTestRunner(workDir string) *TestRunner {
	return &TestRunner{workDir: workDir}
}

func (r *TestRunner) RunTests(language string) (*TestResult, error) {
	color.Blue("Running tests in directory: %s", r.workDir)
	
	var cmd *exec.Cmd
	switch language {
	case "go":
		color.Blue("Running go test -v ./...")
		cmd = exec.Command("go", "test", "-v", "./...")
	case "python":
		color.Blue("Running python -m pytest main_test.py -v")
		cmd = exec.Command("python", "-m", "pytest", "main_test.py", "-v")
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
	
	cmd.Dir = r.workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	output := stdout.String() + stderr.String()
	
	if err != nil {
		color.Yellow("Tests failed: %v", err)
		color.Yellow("Tests failed. Test output:")
		fmt.Println(output)
		return &TestResult{
			Success: false,
			Output:  output,
		}, nil
	}
	
	return &TestResult{
		Success: true,
		Output:  output,
	}, nil
}

func (r *TestRunner) PrepareWorkspace(language string) (string, error) {
	// Create a temporary directory for this run
	tmpDir, err := os.MkdirTemp("", "aiterate-*")
	if err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}
	color.Blue("Created temporary workspace: %s", tmpDir)

	switch language {
	case "go":
		if err := r.initGoModule(tmpDir); err != nil {
			os.RemoveAll(tmpDir) // Clean up on failure
			return "", err
		}
		
		// Create a go.mod file with common dependencies
		goMod := `module temp

go 1.21

require (
	github.com/stretchr/testify v1.8.4
)
`
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("failed to write go.mod: %w", err)
		}

		// Run go mod tidy to download dependencies
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = tmpDir
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		
		if err := cmd.Run(); err != nil {
			color.Red("Failed to run go mod tidy: %v\nOutput: %s\nError: %s",
				err, stdout.String(), stderr.String())
			os.RemoveAll(tmpDir)
			return "", err
		}
	case "python":
		if err := r.initPythonEnv(tmpDir); err != nil {
			return "", err
		}
	}

	return tmpDir, nil
}

func (r *TestRunner) initGoModule(dir string) error {
	color.Blue("Initializing Go module in: %s", dir)
	cmd := exec.Command("go", "mod", "init", "temp")
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		color.Red("Failed to initialize Go module: %v\nOutput: %s\nError: %s",
			err, stdout.String(), stderr.String())
		return err
	}

	color.Green("Successfully initialized Go module")
	return nil
}

func (r *TestRunner) initPythonEnv(dir string) error {
	// Create requirements.txt
	requirementsPath := filepath.Join(dir, "requirements.txt")
	requirements := []byte("pytest>=7.0.0\n")
	if err := os.WriteFile(requirementsPath, requirements, 0644); err != nil {
		return fmt.Errorf("failed to create requirements.txt: %w", err)
	}

	// Install requirements
	color.Blue("Installing Python requirements...")
	cmd := exec.Command("pip", "install", "-r", "requirements.txt")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Python requirements: %v\nOutput: %s\nError: %s",
			err, stdout.String(), stderr.String())
	}

	color.Green("Successfully initialized Python environment")
	return nil
}

func (r *TestRunner) UpdateDependencies(code, testCode string) error {
	color.Blue("Checking for dependencies...")
	
	// Extract import statements using regex
	importRegex := regexp.MustCompile(`import\s*\(([\s\S]*?)\)|\bimport\s+"([^"]+)"`)
	
	// Combine all code for import scanning
	allCode := code + "\n" + testCode
	
	// Find all imports
	matches := importRegex.FindAllStringSubmatch(allCode, -1)
	
	if len(matches) == 0 {
		color.Blue("No external dependencies found")
		return nil
	}

	// Create a map to store unique imports
	imports := make(map[string]bool)
	
	// Process each match
	for _, match := range matches {
		if match[1] != "" {
			// Multi-line import
			lines := strings.Split(match[1], "\n")
			for _, line := range lines {
				pkg := extractPackagePath(line)
				if pkg != "" {
					imports[pkg] = true
				}
			}
		} else if match[2] != "" {
			// Single-line import
			pkg := extractPackagePath(match[2])
			if pkg != "" {
				imports[pkg] = true
			}
		}
	}

	// Update go.mod file
	for pkg := range imports {
		if !isStandardPackage(pkg) {
			color.Blue("Adding dependency: %s", pkg)
			cmd := exec.Command("go", "get", pkg)
			cmd.Dir = r.workDir
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			
			if err := cmd.Run(); err != nil {
				color.Red("Failed to add dependency %s: %v\nOutput: %s\nError: %s",
					pkg, err, stdout.String(), stderr.String())
				return fmt.Errorf("failed to add dependency %s: %w", pkg, err)
			}
		}
	}

	// Run go mod tidy to clean up dependencies
	color.Blue("Running go mod tidy...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = r.workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		color.Red("Failed to run go mod tidy: %v\nOutput: %s\nError: %s",
			err, stdout.String(), stderr.String())
		return fmt.Errorf("failed to run go mod tidy: %w", err)
	}

	color.Green("Dependencies updated successfully")
	return nil
}

func extractPackagePath(line string) string {
	// Remove comments
	if idx := strings.Index(line, "//"); idx != -1 {
		line = line[:idx]
	}
	
	// Clean up the line
	line = strings.TrimSpace(line)
	line = strings.Trim(line, `"`)
	
	// Skip empty lines and dot imports
	if line == "" || line == "." {
		return ""
	}
	
	// Extract the package path
	parts := strings.Fields(line)
	if len(parts) > 0 {
		pkg := parts[len(parts)-1]
		pkg = strings.Trim(pkg, `"`)
		return pkg
	}
	
	return ""
}

func isStandardPackage(pkg string) bool {
	// List of common standard library packages
	stdPkgs := map[string]bool{
		"fmt":      true,
		"os":       true,
		"math":     true,
		"strings":  true,
		"testing":  true,
		"time":     true,
		"net/http": true,
		"encoding/json": true,
		"io":           true,
		"errors":       true,
		"context":      true,
		"sync":         true,
		"bytes":        true,
		"regexp":       true,
	}
	
	// Check if it's a standard package
	if stdPkgs[pkg] {
		return true
	}
	
	// Check if it starts with standard library paths
	return !strings.Contains(pkg, ".")
}
