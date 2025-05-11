package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-wrk/metrics"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.quitting {
		return docStyle.Render("Stopping benchmark if running... Bye!\n")
	}

	var mainContent string

	statusView := m.viewStatus()

	var middleView string
	switch m.status {
	case StatusIdle:
		middleView = m.viewConfig()
	case StatusRunning, StatusStopping, StatusCompleted, StatusError:
		middleView = m.viewMetrics()
	case StatusViewingCollections:
		middleView = m.viewCollectionsList()
	case StatusViewingTests:
		middleView = m.viewTestsList()
	case StatusSelectingMethod:
		middleView = m.viewMethodSelection()
	case StatusSavingEnterCollectionName:
		middleView = m.viewSavingCollectionName()
	case StatusSavingEnterTestName:
		middleView = m.viewSavingTestName()
	default:
		middleView = "Unknown application state."
	}

	topSection := lipgloss.JoinVertical(lipgloss.Left, statusView, middleView)

	vizView := m.viewVisualization()
	logView := m.viewLogs()

	topHeight := lipgloss.Height(topSection)
	helpHeight := lipgloss.Height(m.help.View(m.keys))
	verticalSpacing := 4

	availableHeightForBottom := m.windowHeight - topHeight - helpHeight - verticalSpacing
	if availableHeightForBottom < 6 {
		availableHeightForBottom = 6
	}

	var bottomViews []string
	shouldShowViz := (m.status == StatusRunning || m.status == StatusStopping || m.status == StatusCompleted || m.status == StatusError)

	vizHeight := 0
	if shouldShowViz {
		vizHeight = lipgloss.Height(vizView)
		maxVizHeight := availableHeightForBottom * 2 / 5
		if maxVizHeight < 5 {
			maxVizHeight = 5
		}
		if vizHeight == 0 && availableHeightForBottom > 10 {
			vizHeight = maxVizHeight
		} else if vizHeight > maxVizHeight {
			vizHeight = maxVizHeight
		}
		if vizHeight < 0 {
			vizHeight = 0
		}
	}

	logHeight := availableHeightForBottom - vizHeight
	if logHeight < 3 {
		logHeight = 3
	}

	if !shouldShowViz || vizHeight == 0 {
		logHeight = availableHeightForBottom
		if logHeight < 3 {
			logHeight = 3
		}
	}

	if shouldShowViz && vizHeight > 0 {
		vizViewRendered := panelStyle.Height(vizHeight).MaxHeight(vizHeight).Width(m.windowWidth - 4).Render(vizView)
		bottomViews = append(bottomViews, vizViewRendered)
	}

	logViewRendered := panelStyle.Height(logHeight).MaxHeight(logHeight).Width(m.windowWidth - 4).Render(logView)
	bottomViews = append(bottomViews, logViewRendered)

	bottomSection := lipgloss.JoinVertical(lipgloss.Left, bottomViews...)

	mainContent = lipgloss.JoinVertical(lipgloss.Left,
		topSection,
		bottomSection,
		m.help.View(m.keys),
	)

	return docStyle.Render(mainContent)
}

func (m Model) viewStatus() string {
	var statusLine string
	elapsed := time.Since(m.startTime).Round(time.Second)

	switch m.status {
	case StatusIdle:
		statusLine = statusIdleStyle.Render("Status: Idle (Configuring)")
	case StatusRunning:
		statusLine = statusRunStyle.Render(fmt.Sprintf("Status: Running (Elapsed: %s)", elapsed))
	case StatusStopping:
		statusLine = statusStopStyle.Render("Status: Stopping...")
	case StatusCompleted:
		totalDuration := time.Duration(0)
		if m.finalResult != nil {
			totalDuration = m.finalResult.TotalDuration.Round(time.Millisecond)
		}
		statusLine = statusDoneStyle.Render(fmt.Sprintf("Status: Completed in %s", totalDuration))
	case StatusError:
		errMsg := "Unknown Error"
		if m.currentError != nil {
			errMsg = m.currentError.Error()
		}
		statusLine = statusErrStyle.Render(fmt.Sprintf("Status: Error - %s", errMsg))
	case StatusViewingCollections:
		statusLine = statusIdleStyle.Render("Status: Selecting Collection")
	case StatusViewingTests:
		statusLine = statusIdleStyle.Render("Status: Selecting Test")
	case StatusSelectingMethod:
		statusLine = statusIdleStyle.Render("Status: Selecting Method")
	case StatusSavingEnterCollectionName:
		statusLine = statusIdleStyle.Render("Status: Saving (Enter Collection Name)")
	case StatusSavingEnterTestName:
		statusLine = statusIdleStyle.Render("Status: Saving (Enter Test Name)")
	default:
		statusLine = "Status: Unknown"
	}

	var targetLine string

	if m.status == StatusIdle || m.status == StatusRunning || m.status == StatusStopping ||
		m.status == StatusSavingEnterCollectionName || m.status == StatusSavingEnterTestName {
		method := ""
		if m.selectedMethod >= 0 && m.selectedMethod < len(m.httpMethods) {
			method = m.httpMethods[m.selectedMethod]
		}
		targetLine = fmt.Sprintf("Target: %s %s", method, m.targetURLInput.Value())
	} else if (m.status == StatusCompleted || m.status == StatusError) && m.finalResult != nil && m.finalResult.Config != nil {
		targetLine = fmt.Sprintf("Target: %s %s", m.finalResult.Config.Method, m.finalResult.Config.TargetURL)
	} else {
		targetLine = "Target: -"
	}

	return panelStyle.Width(m.windowWidth - 4).Render(lipgloss.JoinVertical(lipgloss.Left, statusLine, targetLine))
}

func (m Model) viewConfig() string {
	b := strings.Builder{}
	b.WriteString("Benchmark Configuration:\n")

	methodDisplay := "N/A"
	if m.selectedMethod >= 0 && m.selectedMethod < len(m.httpMethods) {
		methodDisplay = m.httpMethods[m.selectedMethod]
	}
	methodLine := fmt.Sprintf("Method: %s", methodDisplay)
	b.WriteString(methodLine + "\n")

	b.WriteString(m.targetURLInput.View() + "\n")
	b.WriteString(m.threadsInput.View() + "\n")
	b.WriteString(m.connectionsInput.View() + "\n")
	b.WriteString(m.durationInput.View() + "\n")

	showPayload := false
	if m.selectedMethod >= 0 && m.selectedMethod < len(m.httpMethods) {
		currentMethod := m.httpMethods[m.selectedMethod]
		if currentMethod == "POST" || currentMethod == "PUT" || currentMethod == "PATCH" {
			showPayload = true
		}
	}

	if showPayload {
		b.WriteString("\n" + m.requestPayload.View() + "\n")
	} else {
		b.WriteString("\n")
	}

	if m.configError != "" {
		b.WriteString(errorStyle.Render(m.configError) + "\n")
	} else if m.saveError != "" {
		b.WriteString(errorStyle.Render(m.saveError) + "\n")
	} else {
		b.WriteString("Press Enter to Start, Ctrl+S to Save, Esc to change method\n")
	}
	return panelStyle.Width(m.windowWidth - 4).Render(b.String())
}

func (m Model) viewSavingCollectionName() string {
	b := strings.Builder{}
	b.WriteString("Save Test Configuration\n\n")
	b.WriteString(m.saveCollectionNameInput.View() + "\n\n")
	if m.saveError != "" {
		b.WriteString(errorStyle.Render(m.saveError) + "\n")
	}
	b.WriteString("Enter collection name (new or existing).\n")
	b.WriteString("Press Enter to continue, Esc to cancel.")
	return panelStyle.Width(m.windowWidth - 4).Render(b.String())
}

func (m Model) viewSavingTestName() string {
	b := strings.Builder{}
	b.WriteString("Save Test Configuration\n\n")
	b.WriteString(fmt.Sprintf("Collection: %s\n", m.saveCollectionNameInput.Value()))
	b.WriteString(m.saveTestNameInput.View() + "\n\n")
	if m.saveError != "" {
		b.WriteString(errorStyle.Render(m.saveError) + "\n")
	}
	b.WriteString("Enter test name (will overwrite if exists).\n")
	b.WriteString("Press Enter to save, Esc to go back to collection name.")
	return panelStyle.Width(m.windowWidth - 4).Render(b.String())
}

func (m Model) viewCollectionsList() string {
	b := strings.Builder{}
	b.WriteString("Select Collection or Start New Benchmark:\n\n")

	for i, collection := range m.testCollections {
		line := collection.Name
		if i == m.selectedCollection {
			b.WriteString(selectedItemStyle.Render("> "+line) + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}

	newBenchmarkLine := "[ New Benchmark ]"
	if m.selectedCollection == len(m.testCollections) {
		b.WriteString(selectedItemStyle.Render("> "+newBenchmarkLine) + "\n")
	} else {
		b.WriteString("  " + newBenchmarkLine + "\n")
	}

	b.WriteString("\nUse ↑↓ to navigate, Enter to select.")
	contentHeight := len(m.testCollections) + 1 + 5
	return panelStyle.Width(m.windowWidth - 4).Height(contentHeight).MaxHeight(m.windowHeight / 3).Render(b.String())
}

func (m Model) viewTestsList() string {
	if len(m.testCollections) == 0 || m.selectedCollection < 0 || m.selectedCollection >= len(m.testCollections) {
		return panelStyle.Width(m.windowWidth - 4).Render("Invalid collection selected or no collections.")
	}
	currentCollection := m.testCollections[m.selectedCollection]
	if len(currentCollection.Tests) == 0 {
		return panelStyle.Width(m.windowWidth - 4).Render(fmt.Sprintf("Collection '%s' has no tests. Press Esc to go back.", currentCollection.Name))
	}

	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("Tests in '%s':\n\n", currentCollection.Name))

	for i, test := range currentCollection.Tests {
		line := test.Name
		if i == m.selectedTest {
			b.WriteString(selectedItemStyle.Render("> "+line) + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}

	b.WriteString("\nUse ↑↓ to navigate, Enter to load, Esc to go back.")
	contentHeight := len(currentCollection.Tests) + 5
	return panelStyle.Width(m.windowWidth - 4).Height(contentHeight).MaxHeight(m.windowHeight / 3).Render(b.String())
}

func (m Model) viewMethodSelection() string {
	b := strings.Builder{}
	b.WriteString("Select HTTP Method:\n\n")

	for i, method := range m.httpMethods {
		if i == m.selectedMethod {
			b.WriteString(selectedItemStyle.Render("> "+method) + "\n")
		} else {
			b.WriteString("  " + method + "\n")
		}
	}

	b.WriteString("\nUse ↑↓ to navigate, Enter to confirm, Esc to go back.")
	contentHeight := len(m.httpMethods) + 5
	return panelStyle.Width(m.windowWidth - 4).Height(contentHeight).MaxHeight(m.windowHeight / 3).Render(b.String())
}

func (m Model) viewMetrics() string {
	b := strings.Builder{}
	title := "Live Metrics"
	data := m.lastProgress

	if m.status == StatusCompleted || m.status == StatusError {
		title = "Final Metrics"
		if m.finalResult != nil {
			data = metrics.ProgressUpdate{
				RequestsAttempted: m.finalResult.TotalRequestsSent,
				RequestsCompleted: m.finalResult.TotalRequestsCompleted,
				Errors:            m.finalResult.TotalErrors,
				CurrentThroughput: m.finalResult.Throughput,
				CurrentErrorRate:  m.finalResult.ErrorRate,
				LatencyAvg:        m.finalResult.LatencyAvg,
				LatencyP95:        m.finalResult.LatencyP95,
				LatencyP99:        m.finalResult.LatencyP99,
			}
		}
	}
	b.WriteString(title + ":\n")

	metricsLines := []string{
		fmt.Sprintf("%s %s", metricKeyStyle.Render("Requests Attempted:"), metricValStyle.Render(strconv.Itoa(data.RequestsAttempted))),
		fmt.Sprintf("%s %s", metricKeyStyle.Render("Requests Completed:"), metricValStyle.Render(strconv.Itoa(data.RequestsCompleted))),
		fmt.Sprintf("%s %s", metricKeyStyle.Render("Errors:"), metricValStyle.Render(strconv.Itoa(data.Errors))),
		fmt.Sprintf("%s %s", metricKeyStyle.Render("Throughput:"), metricValStyle.Render(fmt.Sprintf("%.2f req/sec", data.CurrentThroughput))),
		fmt.Sprintf("%s %s", metricKeyStyle.Render("Error Rate:"), metricValStyle.Render(fmt.Sprintf("%.2f %%", data.CurrentErrorRate))),
		fmt.Sprintf("%s %s", metricKeyStyle.Render("Latency Avg:"), metricValStyle.Render(data.LatencyAvg.Round(time.Millisecond).String())),
		fmt.Sprintf("%s %s", metricKeyStyle.Render("Latency P95:"), metricValStyle.Render(data.LatencyP95.Round(time.Millisecond).String())),
		fmt.Sprintf("%s %s", metricKeyStyle.Render("Latency P99:"), metricValStyle.Render(data.LatencyP99.Round(time.Millisecond).String())),
	}

	if (m.status == StatusCompleted || m.status == StatusError) && m.finalResult != nil && len(m.finalResult.ErrorDetails) > 0 {
		b.WriteString("\nError Summary:\n")
		errorKeys := make([]string, 0, len(m.finalResult.ErrorDetails))
		for k := range m.finalResult.ErrorDetails {
			errorKeys = append(errorKeys, k)
		}
		sort.Strings(errorKeys)
		for _, errKey := range errorKeys {
			count := m.finalResult.ErrorDetails[errKey]
			metricsLines = append(metricsLines, fmt.Sprintf("  %s: %d", errorStyle.Render(errKey), count))
		}
	}

	metricsStr := strings.Join(metricsLines, "\n")
	b.WriteString(metricsStr)

	return panelStyle.Width(m.windowWidth - 4).Render(b.String())
}

func (m Model) viewVisualization() string {
	if m.status != StatusRunning && m.status != StatusStopping && m.status != StatusCompleted && m.status != StatusError {
		return ""
	}

	histWidth := m.windowWidth - 4 - 2
	if histWidth < 20 {
		histWidth = 20
	}

	dataForHist := m.lastProgress.LatencyData
	if (m.status == StatusCompleted || m.status == StatusError) && m.finalResult != nil && len(m.finalResult.LatencyData) > 0 {
		dataForHist = m.finalResult.LatencyData
	}

	if len(dataForHist) == 0 {
		return "Latency Distribution (ms):\nNo latency data yet."
	}

	hist := renderHistogram(dataForHist, histWidth, 8)
	return "Latency Distribution (ms):\n" + hist
}

func (m Model) viewLogs() string {
	logContent := "No logs yet."
	if len(m.logMessages) > 0 {
		logContent = strings.Join(m.logMessages, "\n")
	}

	return logContent
}

func renderHistogram(data []time.Duration, width int, buckets int) string {
	if len(data) == 0 || buckets <= 0 {
		return "No latency data."
	}
	sortedData := make([]time.Duration, len(data))
	copy(sortedData, data)
	metrics.SortLatencies(sortedData)

	minLat, maxLat := sortedData[0], sortedData[len(sortedData)-1]

	if maxLat == minLat {

		minLat = minLat - time.Duration(buckets/2)*time.Millisecond
		maxLat = maxLat + time.Duration(buckets/2+1)*time.Millisecond
		if minLat < 0 {
			minLat = 0
		}
	}
	if maxLat <= minLat {
		maxLat = minLat + time.Duration(buckets)*time.Millisecond
		if maxLat == minLat {
			maxLat += time.Millisecond
		}
	}

	bucketSize := (maxLat - minLat) / time.Duration(buckets)
	if bucketSize <= 0 {
		bucketSize = time.Millisecond
	}

	counts := make([]int, buckets)
	bucketRanges := make([]string, buckets)
	maxCount := 0

	for i := 0; i < buckets; i++ {
		bucketStart := minLat + time.Duration(i)*bucketSize
		bucketEnd := bucketStart + bucketSize

		if i == buckets-1 {
			bucketEnd = maxLat + time.Nanosecond
		}
		bucketRanges[i] = fmt.Sprintf("%4d-%-4d", bucketStart.Milliseconds(), bucketEnd.Milliseconds())

		count := 0
		for _, d := range data {
			if d >= bucketStart && d < bucketEnd {
				count++
			}
		}
		counts[i] = count
		if count > maxCount {
			maxCount = count
		}
	}

	var sb strings.Builder
	labelWidth := 12
	countWidth := 6
	barMaxWidth := width - labelWidth - countWidth - 3
	if barMaxWidth < 1 {
		barMaxWidth = 1
	}

	for i, count := range counts {
		label := fmt.Sprintf("%*s", labelWidth-3, bucketRanges[i]) + "ms"
		barLength := 0
		if maxCount > 0 {
			barLength = int((float64(count) / float64(maxCount)) * float64(barMaxWidth))
		}
		if barLength < 0 {
			barLength = 0
		}
		if barLength > barMaxWidth {
			barLength = barMaxWidth
		}

		bar := strings.Repeat("█", barLength)
		countStr := fmt.Sprintf("[%*d]", countWidth-2, count)
		sb.WriteString(fmt.Sprintf("%s %s: %s\n", label, countStr, histBarStyle.Render(bar)))
	}
	return sb.String()
}
