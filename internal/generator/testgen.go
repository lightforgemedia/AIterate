package generator

import (
	"fmt"

	"github.com/prathyushnallamothu/aiterate/internal/ai"
)

type TestGenerator struct {
	ai *ai.AIClient
}

func NewTestGenerator(ai *ai.AIClient) *TestGenerator {
	return &TestGenerator{ai: ai}
}

func (g *TestGenerator) GenerateTests(description string, language string) (string, error) {
	var prompt string
	switch language {
	case "go":
		prompt = fmt.Sprintf(`Generate comprehensive test cases in Go for the following functionality:
%s

The tests should:
1. Use the "testing" package
2. Include package declaration ("package main" for single file programs)
3. Include all necessary imports
4. Cover normal cases, edge cases, and error conditions
5. Follow Go testing best practices
6. Use descriptive test names (e.g., TestAdd_PositiveNumbers)

Return ONLY the test code without any explanation.`, description)
	case "python":
		prompt = fmt.Sprintf(`Generate comprehensive test cases in Python for the following functionality:
%s

The tests should:
1. Use pytest for testing
2. Include necessary imports (pytest, math, etc.)
3. Cover normal cases, edge cases, and error conditions
4. Follow Python testing best practices
5. Use descriptive test names (e.g., test_add_positive_numbers)
6. Use pytest fixtures if needed
7. Include type hints and docstrings

Return ONLY the test code without any explanation.`, description)
	default:
		prompt = fmt.Sprintf(`Generate comprehensive test cases in %s for the following functionality:
%s

The tests should:
1. Cover normal cases
2. Handle edge cases
3. Test error conditions
4. Follow best practices for %s testing

Return ONLY the test code without any explanation.`, language, description, language)
	}

	code, err := g.ai.GenerateCompletion(prompt)
	if err != nil {
		return "", err
	}

	return stripCodeBlock(code), nil
}
