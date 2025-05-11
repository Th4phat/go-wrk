package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Test struct {
	Name   string
	Config BenchmarkConfig
}

type TestCollection struct {
	Name  string
	Tests []Test
}

type BenchmarkConfig struct {
	TargetURL   string `json:"url"`
	Method      string `json:"method,omitempty"`
	Payload     string `json:"payload,omitempty"`
	Threads     int    `json:"threads"`
	Connections int    `json:"connections"`
	Duration    string `json:"duration"`
}

func (c *BenchmarkConfig) Validate() error {
	c.TargetURL = strings.TrimSpace(c.TargetURL)
	if c.TargetURL == "" {
		return fmt.Errorf("target URL cannot be empty")
	}
	parsedURL, err := url.ParseRequestURI(c.TargetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("target URL must use http or https scheme")
	}

	if c.Threads <= 0 {
		return fmt.Errorf("threads must be greater than 0")
	}
	if c.Connections <= 0 {
		return fmt.Errorf("connections must be greater than 0")
	}
	// if c.Threads > c.Connections {
	//  return fmt.Errorf("threads (%d) should not exceed connections (%d) for optimal use", c.Threads, c.Connections)
	// }

	if c.Duration == "" {
		return fmt.Errorf("duration cannot be empty")
	}
	_, err = time.ParseDuration(c.Duration)
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}

	if (c.Method == "POST" || c.Method == "PUT" || c.Method == "PATCH") && c.Payload == "" {
		// return fmt.Errorf("payload cannot be empty for %s method", c.Method)
	}
	// Add other method/payload validation as needed

	return nil
}

func LoadTestCollections(dirPath string) ([]TestCollection, error) {
	collections := []TestCollection{}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			collectionName := entry.Name()
			collectionPath := filepath.Join(dirPath, collectionName)
			collection := TestCollection{Name: collectionName}

			testEntries, err := os.ReadDir(collectionPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: reading collection directory %s: %v. Skipping.\n", collectionPath, err)
				continue
			}

			for _, testEntry := range testEntries {
				if !testEntry.IsDir() && strings.HasSuffix(testEntry.Name(), ".json") {
					testName := strings.TrimSuffix(testEntry.Name(), ".json")
					testPath := filepath.Join(collectionPath, testEntry.Name())

					content, err := os.ReadFile(testPath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: reading test file %s: %v. Skipping test.\n", testPath, err)
						continue
					}

					var config BenchmarkConfig
					if err := json.Unmarshal(content, &config); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: parsing test file %s: %v. Skipping test.\n", testPath, err)
						continue
					}
					if err := config.Validate(); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: invalid config in test file %s: %v. Skipping test.\n", testPath, err)
						continue
					}
					collection.Tests = append(collection.Tests, Test{Name: testName, Config: config})
				}
			}
			if len(collection.Tests) > 0 {
				collections = append(collections, collection)
			}
		}
	}
	return collections, nil
}

func SaveTestToCollection(
	baseDir string,
	collectionName string,
	testName string,
	cfg BenchmarkConfig,
) error {
	if strings.TrimSpace(collectionName) == "" {
		return fmt.Errorf("collection name cannot be empty")
	}
	if strings.TrimSpace(testName) == "" {
		return fmt.Errorf("test name cannot be empty")
	}
	collectionName = SanitizeFilename(collectionName)
	testName = SanitizeFilename(testName)

	collectionPath := filepath.Join(baseDir, collectionName)
	testFileName := testName + ".json"
	testFilePath := filepath.Join(collectionPath, testFileName)

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
		}
	}

	if _, err := os.Stat(collectionPath); os.IsNotExist(err) {
		if err := os.MkdirAll(collectionPath, 0755); err != nil {
			return fmt.Errorf("failed to create collection directory %s: %w", collectionPath, err)
		}
	}

	if _, err := os.Stat(testFilePath); err == nil {
		// return fmt.Errorf("test file '%s' already exists in collection '%s'", testName, collectionName)
	}

	jsonData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal test config to JSON: %w", err)
	}

	err = os.WriteFile(testFilePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write test file %s: %w", testFilePath, err)
	}

	return nil
}

// SanitizeFilename removes or replaces characters that are problematic in filenames.
func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	replaceChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	for _, char := range replaceChars {
		name = strings.ReplaceAll(name, char, "_")
	}
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			sb.WriteRune(r)
		}
	}
	name = sb.String()
	if name == "" {
		name = "unnamed_test"
	}
	return name
}

func GetConfigDir() string {
	var configDir string
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not determine user config directory: %v. Using ./gowrk instead.\n", err)
		cwd, cerr := os.Getwd()
		if cerr != nil {
			fmt.Fprintf(os.Stderr, "Fatal: Could not get current working directory: %v\n", cerr)
			os.Exit(1)
		}
		configDir = filepath.Join(cwd, "gowrk")
	} else {

		configDir = filepath.Join(userConfigDir, "gowrk")
	}
	return configDir
}
