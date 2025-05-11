package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"go-wrk/benchmark"
	"go-wrk/config"
	"go-wrk/metrics"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type TUIStatus int

const (
	StatusIdle TUIStatus = iota
	StatusRunning
	StatusStopping
	StatusCompleted
	StatusError
	StatusViewingCollections
	StatusViewingTests
	StatusSelectingMethod

	StatusSavingEnterCollectionName
	StatusSavingEnterTestName
)

const maxLogMessages = 100

type Model struct {
	keys keyMap
	help help.Model

	testCollections []config.TestCollection

	targetURLInput   textinput.Model
	threadsInput     textinput.Model
	connectionsInput textinput.Model
	durationInput    textinput.Model
	requestPayload   textinput.Model
	focusedInput     int
	configError      string

	saveCollectionNameInput textinput.Model
	saveTestNameInput       textinput.Model
	currentConfigToSave     *config.BenchmarkConfig
	saveError               string

	httpMethods    []string
	selectedMethod int

	benchmarkEngine *benchmark.Engine
	progressChan    <-chan metrics.ProgressUpdate
	resultChan      <-chan metrics.BenchmarkResult

	status       TUIStatus
	startTime    time.Time
	currentError error

	lastProgress metrics.ProgressUpdate
	finalResult  *metrics.BenchmarkResult

	logMessages  []string
	windowWidth  int
	windowHeight int
	quitting     bool
	logFile      *os.File

	selectedCollection int
	selectedTest       int
}

func NewModel(testCollections []config.TestCollection, logFile *os.File) Model {
	m := Model{
		keys:               keys,
		help:               help.New(),
		testCollections:    testCollections,
		status:             StatusViewingCollections,
		logMessages:        []string{"Welcome! Select a collection or choose [ New Benchmark ]."},
		benchmarkEngine:    benchmark.NewEngine(),
		selectedCollection: 0,
		selectedTest:       0,
		httpMethods:        []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		selectedMethod:     0,
		logFile:            logFile,
	}

	m.targetURLInput = textinput.New()
	m.targetURLInput.Placeholder = "http://example.com/api"
	m.targetURLInput.Prompt = "Target URL: "
	m.targetURLInput.PromptStyle = inputDefaultStyle
	m.targetURLInput.TextStyle = inputDefaultStyle
	m.targetURLInput.PlaceholderStyle = placeholderStyle
	m.targetURLInput.CharLimit = 256
	m.targetURLInput.Width = 50

	m.threadsInput = textinput.New()
	m.threadsInput.Placeholder = "10"
	m.threadsInput.Prompt = "Threads: "
	m.threadsInput.PromptStyle = inputDefaultStyle
	m.threadsInput.TextStyle = inputDefaultStyle
	m.threadsInput.PlaceholderStyle = placeholderStyle
	m.threadsInput.CharLimit = 4
	m.threadsInput.Width = 10
	m.threadsInput.Validate = func(s string) error {
		if s == "" {
			return nil
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if v <= 0 {
			return fmt.Errorf("must be > 0")
		}
		return nil
	}

	m.connectionsInput = textinput.New()
	m.connectionsInput.Placeholder = "50"
	m.connectionsInput.Prompt = "Connections: "
	m.connectionsInput.PromptStyle = inputDefaultStyle
	m.connectionsInput.TextStyle = inputDefaultStyle
	m.connectionsInput.PlaceholderStyle = placeholderStyle
	m.connectionsInput.CharLimit = 4
	m.connectionsInput.Width = 10
	m.connectionsInput.Validate = func(s string) error {
		if s == "" {
			return nil
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if v <= 0 {
			return fmt.Errorf("must be > 0")
		}
		return nil
	}

	m.durationInput = textinput.New()
	m.durationInput.Placeholder = "30s"
	m.durationInput.Prompt = "Duration: "
	m.durationInput.PromptStyle = inputDefaultStyle
	m.durationInput.TextStyle = inputDefaultStyle
	m.durationInput.PlaceholderStyle = placeholderStyle
	m.durationInput.CharLimit = 10
	m.durationInput.Width = 10
	m.durationInput.Validate = func(s string) error {
		if s == "" {
			return nil
		}
		_, err := time.ParseDuration(s)
		return err
	}

	m.requestPayload = textinput.New()
	m.requestPayload.Placeholder = "Enter request payload (JSON for POST/PUT/PATCH)"
	m.requestPayload.Prompt = "Payload: "
	m.requestPayload.PromptStyle = inputDefaultStyle
	m.requestPayload.TextStyle = inputDefaultStyle
	m.requestPayload.PlaceholderStyle = placeholderStyle
	m.requestPayload.CharLimit = 0
	m.requestPayload.Width = 50

	m.saveCollectionNameInput = textinput.New()
	m.saveCollectionNameInput.Placeholder = "my_api_tests"
	m.saveCollectionNameInput.Prompt = "Save to Collection: "
	m.saveCollectionNameInput.PromptStyle = focusedStyle
	m.saveCollectionNameInput.TextStyle = focusedStyle
	m.saveCollectionNameInput.CharLimit = 50
	m.saveCollectionNameInput.Width = 40

	m.saveTestNameInput = textinput.New()
	m.saveTestNameInput.Placeholder = "test_scenario_1"
	m.saveTestNameInput.Prompt = "Save Test Name: "
	m.saveTestNameInput.PromptStyle = focusedStyle
	m.saveTestNameInput.TextStyle = focusedStyle
	m.saveTestNameInput.CharLimit = 50
	m.saveTestNameInput.Width = 40

	m.focusedInput = -1
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) addLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, message)
	m.logMessages = append(m.logMessages, logEntry)
	if len(m.logMessages) > maxLogMessages {
		m.logMessages = m.logMessages[len(m.logMessages)-maxLogMessages:]
	}
	if m.logFile != nil {
		fmt.Fprintln(m.logFile, logEntry)
	}
}

func (m *Model) resetMetricsDisplay() {
	m.lastProgress = metrics.ProgressUpdate{}
	m.finalResult = nil
	m.currentError = nil
}

func (m *Model) clearConfigInputs() {
	m.targetURLInput.SetValue("")
	m.threadsInput.SetValue("")
	m.connectionsInput.SetValue("")
	m.durationInput.SetValue("")
	m.requestPayload.SetValue("")
	m.configError = ""
	m.focusedInput = -1
}

func (m *Model) updateInputFocus() {
	if m.status != StatusIdle &&
		m.status != StatusSavingEnterCollectionName &&
		m.status != StatusSavingEnterTestName {
		m.targetURLInput.Blur()
		m.threadsInput.Blur()
		m.connectionsInput.Blur()
		m.durationInput.Blur()
		m.requestPayload.Blur()
	}
	if m.status == StatusIdle {
		numInputs := 4
		currentMethod := ""
		if m.selectedMethod >= 0 && m.selectedMethod < len(m.httpMethods) {
			currentMethod = m.httpMethods[m.selectedMethod]
		}
		if currentMethod == "POST" || currentMethod == "PUT" || currentMethod == "PATCH" {
			numInputs = 5
		}

		inputs := []*textinput.Model{
			&m.targetURLInput, &m.threadsInput, &m.connectionsInput,
			&m.durationInput, &m.requestPayload,
		}
		for i, input := range inputs {
			shouldFocus := i == m.focusedInput && i < numInputs
			if shouldFocus {
				input.PromptStyle = focusedStyle
				input.TextStyle = focusedStyle
				input.Focus()
			} else {
				input.PromptStyle = inputDefaultStyle
				input.TextStyle = inputDefaultStyle
				input.Blur()
			}
		}
	} else {
		m.targetURLInput.Blur()
		m.threadsInput.Blur()
		m.connectionsInput.Blur()
		m.durationInput.Blur()
		m.requestPayload.Blur()
	}

	if m.status == StatusSavingEnterCollectionName {
		m.saveCollectionNameInput.Focus()
		m.saveTestNameInput.Blur()
	} else if m.status == StatusSavingEnterTestName {
		m.saveCollectionNameInput.Blur()
		m.saveTestNameInput.Focus()
	} else {
		m.saveCollectionNameInput.Blur()
		m.saveTestNameInput.Blur()
	}

	isIdle := m.status == StatusIdle
	numInputs := 4
	currentMethod := ""
	if m.selectedMethod >= 0 && m.selectedMethod < len(m.httpMethods) {
		currentMethod = m.httpMethods[m.selectedMethod]
	}
	if currentMethod == "POST" || currentMethod == "PUT" || currentMethod == "PATCH" {
		numInputs = 5
	}

	inputs := []*textinput.Model{
		&m.targetURLInput,
		&m.threadsInput,
		&m.connectionsInput,
		&m.durationInput,
		&m.requestPayload,
	}

	for i, input := range inputs {
		shouldFocus := isIdle && i == m.focusedInput && i < numInputs
		if shouldFocus {
			input.PromptStyle = focusedStyle
			input.TextStyle = focusedStyle
			input.Focus()
		} else {
			input.PromptStyle = inputDefaultStyle
			input.TextStyle = inputDefaultStyle
			input.Blur()
		}
	}
}

func (m *Model) parseConfig() (config.BenchmarkConfig, error) {
	cfg := config.BenchmarkConfig{}
	var err error

	cfg.TargetURL = m.targetURLInput.Value()

	threadsStr := m.threadsInput.Value()
	if threadsStr == "" {
		return cfg, fmt.Errorf("threads cannot be empty")
	}
	cfg.Threads, err = strconv.Atoi(threadsStr)
	if err != nil {
		return cfg, fmt.Errorf("invalid Threads: %w", err)
	}

	connectionsStr := m.connectionsInput.Value()
	if connectionsStr == "" {
		return cfg, fmt.Errorf("connections cannot be empty")
	}
	cfg.Connections, err = strconv.Atoi(connectionsStr)
	if err != nil {
		return cfg, fmt.Errorf("invalid Connections: %w", err)
	}

	cfg.Duration = m.durationInput.Value()

	if m.selectedMethod < 0 || m.selectedMethod >= len(m.httpMethods) {
		return cfg, fmt.Errorf("invalid HTTP method selected")
	}
	cfg.Method = m.httpMethods[m.selectedMethod]
	cfg.Payload = m.requestPayload.Value()

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	if (cfg.Method == "POST" || cfg.Method == "PUT" || cfg.Method == "PATCH") && cfg.Payload != "" {
		if !isValidJSON(cfg.Payload) {
			m.addLog("Warning: Payload provided but is not valid JSON.")
		}
	}
	return cfg, nil
}

func isValidJSON(s string) bool {
	if s == "" {
		return true
	}
	var jsObj map[string]interface{}
	var jsArr []interface{}
	return json.Unmarshal([]byte(s), &jsObj) == nil || json.Unmarshal([]byte(s), &jsArr) == nil
}
