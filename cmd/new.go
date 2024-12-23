package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/prathyushnallamothu/aiterate/internal/ai"
	"github.com/prathyushnallamothu/aiterate/internal/executor"
	"github.com/prathyushnallamothu/aiterate/internal/generator"
	"github.com/prathyushnallamothu/aiterate/internal/storage"
)

const (
	maxIterations = 5
	storageDir    = ".aiterate"
)

// Supported languages
var supportedLanguages = map[string]bool{
	"go":     true,
	"python": true,
}

func init() {
	rootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new function with AI-generated tests and implementation",
	Long: `Create a new function by describing what you want it to do.
AIterate will:
1. Generate comprehensive tests based on your description
2. Create an implementation that attempts to pass these tests
3. Iteratively improve the code until all tests pass`,
	RunE: runNew,
}

func runNew(cmd *cobra.Command, args []string) error {
	// Initialize components
	aiClient, err := ai.NewAIClient()
	if err != nil {
		return fmt.Errorf("failed to initialize AI client: %w", err)
	}

	testGen := generator.NewTestGenerator(aiClient)
	codeGen := generator.NewCodeGenerator(aiClient)
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	
	store, err := storage.NewStorage(filepath.Join(homeDir, storageDir))
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	var description string
	if len(args) > 0 {
		description = args[0]
	} else {
		fmt.Print("Enter a description of the function you want to create: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			description = scanner.Text()
		}
	}

	if description == "" {
		return fmt.Errorf("description is required")
	}

	// Get programming language
	fmt.Print("Enter the programming language (e.g., go, python): ")
	scanner := bufio.NewScanner(os.Stdin)
	var language string
	if scanner.Scan() {
		language = strings.ToLower(strings.TrimSpace(scanner.Text()))
	}

	if language == "" {
		return fmt.Errorf("language is required")
	}

	// Validate language
	if !supportedLanguages[language] {
		return fmt.Errorf("unsupported language: %s. Supported languages: go, python", language)
	}

	// Create new session
	session, err := store.CreateSession(description, language)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Create test runner with temporary workspace
	runner := executor.NewTestRunner("")
	workDir, err := runner.PrepareWorkspace(language)
	if err != nil {
		return fmt.Errorf("failed to prepare workspace: %w", err)
	}
	defer os.RemoveAll(workDir)
	runner = executor.NewTestRunner(workDir)

	// Create output directory with AI-generated name
	outputDirName, err := codeGen.GenerateDirectoryName(description)
	if err != nil {
		outputDirName = "generated-function" // fallback name
	}
	
	// Create the output directory
	outputDir := filepath.Join(".", outputDirName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	color.Blue("Created output directory: %s", outputDir)

	// Generate tests
	color.Blue("Generating tests...")
	testCode, err := testGen.GenerateTests(description, language)
	if err != nil {
		return fmt.Errorf("failed to generate tests: %w", err)
	}

	// Generate initial implementation
	color.Blue("Generating initial implementation...")
	code, err := codeGen.GenerateImplementation(description, testCode, language)
	if err != nil {
		return fmt.Errorf("failed to generate implementation: %w", err)
	}

	// Save test and implementation files
	if err := writeFiles(workDir, testCode, code, language); err != nil {
		return fmt.Errorf("failed to write files: %w", err)
	}

	// Iteration loop
	var success bool
	var lastTestOutput string
	for i := 0; i < maxIterations; i++ {
		color.Blue("Running tests (iteration %d/%d)...", i+1, maxIterations)
		
		result, err := runner.RunTests(language)
		if err != nil {
			return fmt.Errorf("failed to run tests: %w", err)
		}

		lastTestOutput = result.Output
		// Store iteration
		if err := store.AddIteration(session.ID, testCode, code, result.Output, result.Success); err != nil {
			return fmt.Errorf("failed to store iteration: %w", err)
		}

		if result.Success {
			success = true
			color.Green("All tests passed!")
			break
		}

		color.Yellow("Tests failed. Test output:")
		fmt.Println(result.Output)
		color.Yellow("Attempting to fix implementation and tests...")
		
		// Fix both implementation and tests
		fixResult, err := codeGen.FixBoth(code, testCode, result.Output, language)
		if err != nil {
			return fmt.Errorf("failed to fix code: %w", err)
		}

		// Update both files
		code = fixResult.Code
		testCode = fixResult.TestCode
		
		if err := writeFiles(workDir, testCode, code, language); err != nil {
			return fmt.Errorf("failed to write files: %w", err)
		}
	}

	// Always copy files, even if tests didn't pass
	if err := copyFinalFiles(workDir, outputDir, language); err != nil {
		return fmt.Errorf("failed to copy final files: %w", err)
	}

	if !success {
		color.Red("Failed to generate passing implementation after %d iterations", maxIterations)
		color.Yellow("Last test output:")
		fmt.Println(lastTestOutput)
		color.Yellow("Files have been saved to: %s", outputDir)
	} else {
		color.Green("Successfully generated code! Check %s for the files.", outputDir)
	}
	return nil
}

func getFileExtension(language string) string {
	switch language {
	case "python":
		return "py"
	case "go":
		return "go"
	default:
		return ""
	}
}

func writeFiles(dir, testCode, code, language string) error {
	color.Blue("Writing files to temporary directory: %s", dir)
	
	ext := getFileExtension(language)
	if ext == "" {
		return fmt.Errorf("unsupported language: %s", language)
	}
	
	// Write test file
	testFile := filepath.Join(dir, fmt.Sprintf("main_test.%s", ext))
	color.Blue("Writing test file: %s", testFile)
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}

	// Write implementation file
	implFile := filepath.Join(dir, fmt.Sprintf("main.%s", ext))
	color.Blue("Writing implementation file: %s", implFile)
	if err := os.WriteFile(implFile, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write implementation file: %w", err)
	}

	// Update dependencies if it's a Go project
	if language == "go" {
		runner := executor.NewTestRunner(dir)
		if err := runner.UpdateDependencies(code, testCode); err != nil {
			return fmt.Errorf("failed to update dependencies: %w", err)
		}
	}

	return nil
}

func writeImplementation(dir, code, language string) error {
	ext := getFileExtension(language)
	if ext == "" {
		return fmt.Errorf("unsupported language: %s", language)
	}
	
	implFile := filepath.Join(dir, fmt.Sprintf("main.%s", ext))
	color.Blue("Updating implementation file: %s", implFile)
	return os.WriteFile(implFile, []byte(code), 0644)
}

func copyFinalFiles(srcDir, dstDir, language string) error {
	color.Blue("Copying files from %s to %s", srcDir, dstDir)
	
	ext := getFileExtension(language)
	if ext == "" {
		return fmt.Errorf("unsupported language: %s", language)
	}
	
	files := []string{
		fmt.Sprintf("main_test.%s", ext),
		fmt.Sprintf("main.%s", ext),
	}

	for _, file := range files {
		src := filepath.Join(srcDir, file)
		dst := filepath.Join(dstDir, file)
		
		color.Blue("Reading from: %s", src)
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}
		
		color.Blue("Writing to: %s", dst)
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", file, err)
		}
		color.Green("Successfully copied %s", file)
	}

	return nil
}
