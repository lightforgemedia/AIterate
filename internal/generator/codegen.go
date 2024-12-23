package generator

import (
	"fmt"
	"strings"

	"github.com/prathyushnallamothu/aiterate/internal/ai"
)

type CodeGenerator struct {
	ai *ai.AIClient
}

func NewCodeGenerator(ai *ai.AIClient) *CodeGenerator {
	return &CodeGenerator{ai: ai}
}

func stripCodeBlock(code string) string {
	// Remove leading and trailing whitespace
	code = strings.TrimSpace(code)

	// If the code starts with a code block marker, remove it and the language identifier
	if strings.HasPrefix(code, "```") {
		lines := strings.Split(code, "\n")
		if len(lines) <= 1 {
			return "" // Return empty string if there's only the opening marker
		}
		
		// Remove the first line (opening marker with language)
		lines = lines[1:]
		
		// Find and remove the closing marker if it exists
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) == "```" {
				lines = lines[:i]
				break
			}
		}
		
		code = strings.Join(lines, "\n")
	}

	return strings.TrimSpace(code)
}

func (g *CodeGenerator) GenerateImplementation(description string, testCode string, language string) (string, error) {
	var prompt string
	switch language {
	case "go":
		prompt = fmt.Sprintf(`Given these Go tests:
%s

Generate a Go implementation that passes all tests. The implementation should:
1. Include package declaration ("package main" for single file programs)
2. Include all necessary imports
3. Handle all test cases including edge cases
4. Follow Go best practices
5. Include error handling
6. Include comments for exported functions

Return ONLY the implementation code without any explanation.`, testCode)
	case "python":
		prompt = fmt.Sprintf(`Given these Python tests:
%s

Generate a Python implementation that passes all tests. The implementation should:
1. Include all necessary imports
2. Use type hints for function parameters and return values
3. Include proper docstrings
4. Handle all test cases including edge cases
5. Follow PEP 8 style guidelines
6. Include error handling
7. Use modern Python features (f-strings, walrus operator where appropriate)

Return ONLY the implementation code without any explanation.`, testCode)
	default:
		prompt = fmt.Sprintf(`Given these %s tests:
%s

Generate an implementation that passes all tests. The implementation should:
1. Include all necessary imports
2. Handle all test cases
3. Follow best practices
4. Include error handling

Return ONLY the implementation code without any explanation.`, language, testCode)
	}

	code, err := g.ai.GenerateCompletion(prompt)
	if err != nil {
		return "", err
	}

	return stripCodeBlock(code), nil
}

func (g *CodeGenerator) FixImplementation(currentCode string, testCode string, testOutput string, language string) (string, error) {
	prompt := fmt.Sprintf(`The following %s code failed some tests:

Current Implementation:
%s

Test Code:
%s

Test Output (errors):
%s

Fix the implementation to make all tests pass. Return ONLY the fixed implementation code without any explanation.`, language, currentCode, testCode, testOutput)

	code, err := g.ai.GenerateCompletion(prompt)
	if err != nil {
		return "", err
	}

	return stripCodeBlock(code), nil
}

func (g *CodeGenerator) GenerateDirectoryName(description string) (string, error) {
	prompt := fmt.Sprintf(`Given this function description:
"%s"

Generate a short, descriptive directory name that:
1. Is under 30 characters
2. Uses only lowercase letters, numbers, and hyphens
3. Describes the main purpose of the function
4. Starts with a letter

Return ONLY the directory name, nothing else.`, description)

	name, err := g.ai.GenerateCompletion(prompt)
	if err != nil {
		return "", err
	}

	// Clean up the generated name
	name = stripCodeBlock(name)
	name = strings.ToLower(strings.TrimSpace(name))
	
	// Replace any invalid characters with hyphens
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	
	// Remove consecutive hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	
	// Trim hyphens from ends
	name = strings.Trim(name, "-")
	
	// Ensure it starts with a letter
	if len(name) > 0 && !((name[0] >= 'a' && name[0] <= 'z')) {
		name = "fn-" + name
	}

	return name, nil
}

type FixResult struct {
	TestCode string
	Code     string
}

func (g *CodeGenerator) FixBoth(currentCode, currentTestCode string, testOutput string, language string) (*FixResult, error) {
	prompt := fmt.Sprintf(`The following %s code and tests failed:

Current Implementation:
%s

Current Test Code:
%s

Test Output (errors):
%s

Fix BOTH the implementation and test code to make all tests pass. Return the fixed code in this exact format:

---IMPLEMENTATION---
[Your fixed implementation code here]
---TESTS---
[Your fixed test code here]
---END---`, language, currentCode, currentTestCode, testOutput)

	response, err := g.ai.GenerateCompletion(prompt)
	if err != nil {
		return nil, err
	}

	// Parse the response to get both implementation and test code
	parts := strings.Split(response, "---")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid response format from AI")
	}

	var implementation, tests string
	for i, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "IMPLEMENTATION":
			if i+1 < len(parts) {
				implementation = stripCodeBlock(parts[i+1])
			}
		case "TESTS":
			if i+1 < len(parts) {
				tests = stripCodeBlock(parts[i+1])
			}
		}
	}

	if implementation == "" || tests == "" {
		return nil, fmt.Errorf("failed to extract implementation or test code")
	}

	return &FixResult{
		TestCode: tests,
		Code:     implementation,
	}, nil
}
