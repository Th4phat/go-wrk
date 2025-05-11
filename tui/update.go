package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Th4phat/go-wrk/benchmark"
	"github.com/Th4phat/go-wrk/config"
	"github.com/Th4phat/go-wrk/metrics"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type progressMsg metrics.ProgressUpdate
type resultMsg metrics.BenchmarkResult
type benchmarkCompleteMsg struct{}

func listenForProgress(progressChan <-chan metrics.ProgressUpdate) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-progressChan
		if !ok {

			return nil
		}
		return progressMsg(update)
	}
}

func listenForResult(resultChan <-chan metrics.BenchmarkResult) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-resultChan
		if !ok {

			return benchmarkCompleteMsg{}
		}

		return resultMsg(res)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var _ tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.help.Width = msg.Width

	case tea.KeyMsg:

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			if m.status == StatusRunning || m.status == StatusStopping {
				m.addLog("Quit key: Stopping benchmark engine...")
				m.benchmarkEngine.Stop()
				m.status = StatusStopping
			} else {
				m.addLog("Quit key: Exiting.")
			}
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil

		case key.Matches(msg, m.keys.Refresh):
			m.addLog("Refresh key: Resetting UI.")
			if m.status == StatusRunning || m.status == StatusStopping {
				m.addLog("Refresh: Stopping benchmark engine...")
				m.benchmarkEngine.Stop()
			}
			m.clearConfigInputs()
			m.resetMetricsDisplay()
			m.configError = ""
			m.saveError = ""
			m.currentConfigToSave = nil
			m.focusedInput = -1
			m.progressChan = nil
			m.resultChan = nil
			m.status = StatusViewingCollections
			m.selectedCollection = 0
			m.selectedTest = 0
			m.selectedMethod = 0
			m.benchmarkEngine = benchmark.NewEngine()
			m.logMessages = []string{"UI Refreshed. Select a collection or [ New Benchmark ]."}

			return m, nil
		}

		if m.status == StatusIdle && key.Matches(msg, m.keys.Save) {
			m.addLog("Save key (Ctrl+S) pressed in Idle state.")
			m.configError = ""
			m.saveError = ""
			parsedCfg, err := m.parseConfig()
			if err != nil {
				m.configError = fmt.Sprintf("Cannot save: Config Error: %v", err)
				m.addLog(m.configError)

			} else {
				m.currentConfigToSave = &parsedCfg
				m.status = StatusSavingEnterCollectionName
				m.saveCollectionNameInput.SetValue("")
				m.saveTestNameInput.SetValue("")
				m.addLog("Please enter collection name to save to.")

			}

		}

		var statusChangeCmd tea.Cmd
		originalStatus := m.status

		switch m.status {
		case StatusIdle:

			if !key.Matches(msg, m.keys.Save) {
				statusChangeCmd = m.handleIdleKeys(msg, &cmds)
			}
		case StatusRunning:
			statusChangeCmd = m.handleRunningKeys(msg, &cmds)
		case StatusStopping:
			m.addLog("Ignoring key press while stopping.")
		case StatusCompleted, StatusError:
			statusChangeCmd = m.handleFinishedKeys(msg, &cmds)
		case StatusViewingCollections:
			statusChangeCmd = m.handleViewingCollectionsKeys(msg)
		case StatusViewingTests:
			statusChangeCmd = m.handleViewingTestsKeys(msg)
		case StatusSelectingMethod:
			statusChangeCmd = m.handleSelectingMethodKeys(msg)
		case StatusSavingEnterCollectionName:
			statusChangeCmd = m.handleSavingCollectionNameKeys(msg)
		case StatusSavingEnterTestName:
			statusChangeCmd = m.handleSavingTestNameKeys(msg)
		}

		if statusChangeCmd != nil {
			cmds = append(cmds, statusChangeCmd)
		}
		if m.status != originalStatus {
			m.addLog(fmt.Sprintf("Status changed by key (%s): %v -> %v", msg.String(), originalStatus, m.status))
		}

	case progressMsg:
		if m.progressChan != nil && (m.status == StatusRunning || m.status == StatusStopping) {
			m.lastProgress = metrics.ProgressUpdate(msg)
		}
	case resultMsg:
		m.addLog("Received resultMsg.")
		if m.resultChan != nil && (m.status == StatusRunning || m.status == StatusStopping) {
			finalResult := metrics.BenchmarkResult(msg)
			m.finalResult = &finalResult
			if finalResult.Error != nil {
				m.addLog("resultMsg contains error. Setting StatusError.")
				m.status = StatusError
				m.currentError = finalResult.Error
				m.addLog(fmt.Sprintf("Benchmark error logged: %v", finalResult.Error))
			} else {
				m.addLog("resultMsg successful. Setting StatusCompleted.")
				m.status = StatusCompleted
				duration := finalResult.TotalDuration.Round(time.Millisecond)
				m.addLog(successStyle.Render(fmt.Sprintf("Benchmark completed in %s.", duration)))
			}
			m.lastProgress = metrics.ProgressUpdate{
				RequestsAttempted: finalResult.TotalRequestsSent, RequestsCompleted: finalResult.TotalRequestsCompleted,
				Errors: finalResult.TotalErrors, CurrentThroughput: finalResult.Throughput, CurrentErrorRate: finalResult.ErrorRate,
				LatencyAvg: finalResult.LatencyAvg, LatencyP95: finalResult.LatencyP95, LatencyP99: finalResult.LatencyP99,
				LatencyData: finalResult.LatencyData,
			}
			m.addLog("Nil-ing progressChan and resultChan after resultMsg.")
			m.progressChan = nil
			m.resultChan = nil
		}
	case benchmarkCompleteMsg:
		if m.resultChan != nil && (m.status == StatusRunning || m.status == StatusStopping) {
			m.addLog("benchmarkCompleteMsg relevant. Setting StatusCompleted.")
			m.status = StatusCompleted
			duration := time.Since(m.startTime).Round(time.Millisecond)
			if m.finalResult != nil {
				duration = m.finalResult.TotalDuration.Round(time.Millisecond)
			}
			m.addLog(successStyle.Render(fmt.Sprintf("Benchmark completed (result chan closed) in %s.", duration)))
			m.addLog("Nil-ing progressChan and resultChan after benchmarkCompleteMsg.")
			m.progressChan = nil
			m.resultChan = nil
		}

	}

	var textInputCmd tea.Cmd
	if keyMsg, ok := msg.(tea.KeyMsg); ok {

		isActionKey := false
		switch m.status {
		case StatusIdle:
			isActionKey = key.Matches(keyMsg, m.keys.Start, m.keys.Back, m.keys.Up, m.keys.Down, m.keys.Save)
		case StatusSavingEnterCollectionName, StatusSavingEnterTestName:
			isActionKey = key.Matches(keyMsg, m.keys.Enter, m.keys.Back)

		}

		if !isActionKey {
			switch m.status {
			case StatusIdle:

				textInputCmd = m.updateFocusedInput(keyMsg)
			case StatusSavingEnterCollectionName:
				m.saveCollectionNameInput, textInputCmd = m.saveCollectionNameInput.Update(keyMsg)
			case StatusSavingEnterTestName:
				m.saveTestNameInput, textInputCmd = m.saveTestNameInput.Update(keyMsg)
			}
		}
	}
	if textInputCmd != nil {
		cmds = append(cmds, textInputCmd)
	}

	shouldBeListening := m.status == StatusRunning || m.status == StatusStopping
	if shouldBeListening {
		if m.progressChan != nil {
			cmds = append(cmds, listenForProgress(m.progressChan))
		}
		if m.resultChan != nil {
			cmds = append(cmds, listenForResult(m.resultChan))
		}
	}

	m.updateInputFocus()

	return m, tea.Batch(cmds...)
}

func (m *Model) handleRunningKeys(msg tea.KeyMsg, _ *[]tea.Cmd) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Stop):

		if m.status == StatusRunning {
			m.addLog("Stop key pressed while Running. Calling benchmarkEngine.Stop().")
			m.benchmarkEngine.Stop()
			m.status = StatusStopping
			m.addLog("Status set to StatusStopping.")

		} else {
			m.addLog("Stop key pressed but status was not Running.")
		}
	}
	return nil
}

func (m *Model) handleIdleKeys(msg tea.KeyMsg, cmds *[]tea.Cmd) tea.Cmd {
	var cmd tea.Cmd
	numInputs := 4
	currentMethod := ""

	if m.selectedMethod >= 0 && m.selectedMethod < len(m.httpMethods) {
		currentMethod = m.httpMethods[m.selectedMethod]
	} else {
		m.addLog(fmt.Sprintf("Warning: Invalid selectedMethod index %d in handleIdleKeys", m.selectedMethod))

		if len(m.httpMethods) > 0 {
			m.selectedMethod = 0
			currentMethod = m.httpMethods[0]
		}
	}

	if currentMethod == "POST" || currentMethod == "PUT" || currentMethod == "PATCH" {
		numInputs = 5
	}

	switch {
	case key.Matches(msg, m.keys.Start):
		m.configError = ""
		m.saveError = ""
		m.addLog("Start key pressed in Idle. Parsing config...")
		cfg, err := m.parseConfig()
		if err != nil {
			m.configError = fmt.Sprintf("Config Error: %v", err)
			m.addLog(m.configError)
		} else {
			m.addLog(fmt.Sprintf("Config parsed. Starting benchmark: %s %s (%d threads, %d conns, %s)",
				cfg.Method, cfg.TargetURL, cfg.Threads, cfg.Connections, cfg.Duration))

			m.resetMetricsDisplay()
			m.startTime = time.Now()
			m.focusedInput = -1

			progressChan := make(chan metrics.ProgressUpdate)
			resultChan := make(chan metrics.BenchmarkResult)
			m.progressChan = progressChan
			m.resultChan = resultChan
			m.addLog("Created new progress and result channels.")

			err = m.benchmarkEngine.Start(cfg, progressChan, resultChan)

			if err != nil {
				m.status = StatusError
				m.currentError = err
				m.addLog(fmt.Sprintf("FATAL: Failed to start benchmark engine: %v", err))
				close(progressChan)
				close(resultChan)
				m.progressChan = nil
				m.resultChan = nil
				m.addLog("Cleaned up channels due to start error.")
			} else {
				m.status = StatusRunning
				m.addLog("Benchmark engine started successfully. Status set to Running.")
				*cmds = append(*cmds, listenForProgress(m.progressChan))
				*cmds = append(*cmds, listenForResult(m.resultChan))
				m.addLog("Added progress and result listeners.")
			}
		}

	case key.Matches(msg, m.keys.Down):
		if m.focusedInput < 0 {
			m.focusedInput = 0
		} else {
			m.focusedInput = (m.focusedInput + 1) % numInputs
		}
		m.updateInputFocus()
		cmd = textinput.Blink

	case key.Matches(msg, m.keys.Up):
		if m.focusedInput < 0 {
			m.focusedInput = numInputs - 1
		} else {
			m.focusedInput = (m.focusedInput - 1 + numInputs) % numInputs
		}
		m.updateInputFocus()
		cmd = textinput.Blink

	case key.Matches(msg, m.keys.Back):
		m.status = StatusSelectingMethod
		m.addLog("Back key in Idle: Returning to method selection.")
		m.configError = ""
		m.saveError = ""
		m.focusedInput = -1

	default:

	}
	return cmd
}
func (m *Model) handleSavingCollectionNameKeys(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Enter):
		collectionName := strings.TrimSpace(m.saveCollectionNameInput.Value())
		if collectionName == "" {
			m.saveError = "Collection name cannot be empty."
			m.addLog(m.saveError)
			return textinput.Blink
		}
		m.saveError = ""
		m.status = StatusSavingEnterTestName
		m.addLog(fmt.Sprintf("Collection: '%s'. Now enter test name.", collectionName))
		return textinput.Blink
	case key.Matches(msg, m.keys.Back):
		m.status = StatusIdle
		m.currentConfigToSave = nil
		m.saveError = ""
		m.addLog("Save cancelled. Returned to Idle state.")

	}
	return nil
}

func (m *Model) handleSavingTestNameKeys(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Enter):
		collectionName := strings.TrimSpace(m.saveCollectionNameInput.Value())
		testName := strings.TrimSpace(m.saveTestNameInput.Value())
		if testName == "" {
			m.saveError = "Test name cannot be empty."
			m.addLog(m.saveError)
			return textinput.Blink
		}
		if m.currentConfigToSave == nil {
			m.saveError = "Error: No configuration to save."
			m.addLog(m.saveError)
			m.status = StatusIdle
			return nil
		}

		m.addLog(fmt.Sprintf("Attempting to save test '%s' to collection '%s'...", testName, collectionName))

		configDir := config.GetConfigDir()
		err := config.SaveTestToCollection(configDir, collectionName, testName, *m.currentConfigToSave)
		if err != nil {
			m.saveError = fmt.Sprintf("Save failed: %v", err)
			m.addLog(m.saveError)

			return textinput.Blink
		}

		m.addLog(successStyle.Render(fmt.Sprintf("Successfully saved test '%s' to collection '%s'.", testName, collectionName)))
		m.status = StatusIdle
		m.currentConfigToSave = nil
		m.saveError = ""

		return nil

	case key.Matches(msg, m.keys.Back):
		m.status = StatusSavingEnterCollectionName
		m.saveError = ""
		m.addLog("Back to entering collection name.")
		return textinput.Blink
	}
	return nil
}
func (m *Model) handleViewingTestsKeys(msg tea.KeyMsg) tea.Cmd {
	if len(m.testCollections) == 0 || m.selectedCollection < 0 || m.selectedCollection >= len(m.testCollections) {
		m.addLog("Warning: handleViewingTestsKeys called with invalid selectedCollection.")
		m.status = StatusViewingCollections
		return nil
	}
	currentCollection := m.testCollections[m.selectedCollection]
	if len(currentCollection.Tests) == 0 {
		if key.Matches(msg, m.keys.Back) {
			m.status = StatusViewingCollections
			m.selectedTest = 0
			m.addLog("Returning to collections view (no tests in selected collection).")
		}
		return nil
	}

	switch {
	case key.Matches(msg, m.keys.Down):
		m.selectedTest = (m.selectedTest + 1) % len(currentCollection.Tests)
	case key.Matches(msg, m.keys.Up):
		m.selectedTest = (m.selectedTest - 1 + len(currentCollection.Tests)) % len(currentCollection.Tests)
	case key.Matches(msg, m.keys.Enter):
		if m.selectedTest < 0 || m.selectedTest >= len(currentCollection.Tests) {
			m.addLog(fmt.Sprintf("Warning: Invalid selectedTest index %d in handleViewingTestsKeys", m.selectedTest))
			return nil
		}
		selectedTest := currentCollection.Tests[m.selectedTest]

		m.targetURLInput.SetValue(selectedTest.Config.TargetURL)
		m.threadsInput.SetValue(strconv.Itoa(selectedTest.Config.Threads))
		m.connectionsInput.SetValue(strconv.Itoa(selectedTest.Config.Connections))
		m.durationInput.SetValue(selectedTest.Config.Duration)
		m.requestPayload.SetValue(selectedTest.Config.Payload)

		m.selectedMethod = 0
		if selectedTest.Config.Method != "" {
			found := false
			for i, method := range m.httpMethods {
				if strings.EqualFold(method, selectedTest.Config.Method) {
					m.selectedMethod = i
					found = true
					break
				}
			}
			if !found {
				m.addLog(fmt.Sprintf("Warning: Method '%s' from test '%s' not found. Defaulting to GET.", selectedTest.Config.Method, selectedTest.Name))
			}
		}

		m.status = StatusIdle
		m.addLog(fmt.Sprintf("Loaded test '%s' from '%s'. Ready to start or modify.", selectedTest.Name, currentCollection.Name))
		m.configError = ""
		m.focusedInput = 0
		return textinput.Blink

	case key.Matches(msg, m.keys.Back):
		m.status = StatusViewingCollections
		m.selectedTest = 0
		m.addLog("Returning to collections view.")
	}
	return nil
}

func (m *Model) updateFocusedInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if m.focusedInput < 0 {
		return nil
	}
	switch m.focusedInput {
	case 0:
		m.targetURLInput, cmd = m.targetURLInput.Update(msg)
	case 1:
		m.threadsInput, cmd = m.threadsInput.Update(msg)
	case 2:
		m.connectionsInput, cmd = m.connectionsInput.Update(msg)
	case 3:
		m.durationInput, cmd = m.durationInput.Update(msg)
	case 4:
		m.requestPayload, cmd = m.requestPayload.Update(msg)
	default:
		m.addLog(fmt.Sprintf("Warning: updateFocusedInput called with invalid index %d", m.focusedInput))
	}
	return cmd
}

func (m *Model) handleViewingCollectionsKeys(msg tea.KeyMsg) tea.Cmd {
	totalOptions := len(m.testCollections) + 1

	switch {
	case key.Matches(msg, m.keys.Down):
		m.selectedCollection = (m.selectedCollection + 1) % totalOptions
	case key.Matches(msg, m.keys.Up):
		m.selectedCollection = (m.selectedCollection - 1 + totalOptions) % totalOptions
	case key.Matches(msg, m.keys.Enter):
		if m.selectedCollection == len(m.testCollections) {
			m.status = StatusSelectingMethod
			m.clearConfigInputs()
			m.selectedMethod = 0
			m.addLog("Selected [ New Benchmark ]. Choose HTTP method.")
		} else if len(m.testCollections) > 0 && m.selectedCollection < len(m.testCollections) {

			m.status = StatusViewingTests
			m.selectedTest = 0
			m.addLog(fmt.Sprintf("Viewing tests in collection: %s", m.testCollections[m.selectedCollection].Name))
		}
	}
	return nil
}

func (m *Model) handleSelectingMethodKeys(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Down):
		m.selectedMethod = (m.selectedMethod + 1) % len(m.httpMethods)
	case key.Matches(msg, m.keys.Up):
		m.selectedMethod = (m.selectedMethod - 1 + len(m.httpMethods)) % len(m.httpMethods)
	case key.Matches(msg, m.keys.Enter):
		m.status = StatusIdle
		m.focusedInput = 0
		m.addLog(fmt.Sprintf("Selected method: %s. Configure benchmark details.", m.httpMethods[m.selectedMethod]))
		return textinput.Blink
	case key.Matches(msg, m.keys.Back):
		m.status = StatusViewingCollections
		m.addLog("Returning to collections view from method selection.")

		m.clearConfigInputs()
		m.selectedMethod = 0
	}
	return nil
}

func (m *Model) handleFinishedKeys(msg tea.KeyMsg, _ *[]tea.Cmd) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Start):
		m.status = StatusIdle
		m.resetMetricsDisplay()
		m.configError = ""
		m.addLog("Ready for new benchmark configuration.")
		m.focusedInput = 0
		m.updateInputFocus()
		return textinput.Blink
	}
	return nil
}
