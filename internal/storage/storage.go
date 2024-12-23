package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type Iteration struct {
	Number    int       `json:"number"`
	TestCode  string    `json:"test_code"`
	Code      string    `json:"code"`
	TestLogs  string    `json:"test_logs"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
}

type Session struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Language    string      `json:"language"`
	Iterations  []Iteration `json:"iterations"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Storage struct {
	baseDir string
}

func NewStorage(baseDir string) (*Storage, error) {
	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &Storage{baseDir: baseDir}, nil
}

func (s *Storage) CreateSession(description, language string) (*Session, error) {
	session := &Session{
		ID:          uuid.New().String(),
		Description: description,
		Language:    language,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Create session directory
	sessionDir := filepath.Join(s.baseDir, session.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Save session metadata
	if err := s.saveSession(session); err != nil {
		return nil, err
	}

	return session, nil
}

func (s *Storage) AddIteration(sessionID string, testCode, code, testLogs string, success bool) error {
	session, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	iteration := Iteration{
		Number:    len(session.Iterations) + 1,
		TestCode:  testCode,
		Code:      code,
		TestLogs:  testLogs,
		Success:   success,
		Timestamp: time.Now(),
	}

	session.Iterations = append(session.Iterations, iteration)
	session.UpdatedAt = time.Now()

	return s.saveSession(session)
}

func (s *Storage) GetSession(sessionID string) (*Session, error) {
	data, err := os.ReadFile(filepath.Join(s.baseDir, sessionID, "session.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %w", err)
	}

	return &session, nil
}

func (s *Storage) saveSession(session *Session) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	filename := filepath.Join(s.baseDir, session.ID, "session.json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to save session data: %w", err)
	}

	return nil
}
