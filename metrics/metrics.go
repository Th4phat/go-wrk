package metrics

import (
	"fmt"
	"math"
	"sort"
	"time"

	config "github.com/Th4phat/go-wrk/config"
)

// ProgressUpdate holds metrics for a point-in-time update during the benchmark.
type ProgressUpdate struct {
	Timestamp         time.Time
	RequestsAttempted int
	RequestsCompleted int
	Errors            int
	CurrentThroughput float64         // Instantaneous or short-window throughput
	CurrentErrorRate  float64         // Instantaneous or short-window error rate
	LatencyAvg        time.Duration   // Cumulative Avg
	LatencyP95        time.Duration   // Cumulative P95
	LatencyP99        time.Duration   // Cumulative P99
	LatencyData       []time.Duration // Recent raw data for live histogram
}

// BenchmarkResult holds the final aggregated results of a benchmark run.
type BenchmarkResult struct {
	Config                 *config.BenchmarkConfig // Include config used
	TotalRequestsSent      int
	TotalRequestsCompleted int
	TotalErrors            int
	TotalDuration          time.Duration
	Throughput             float64
	ErrorRate              float64
	LatencyAvg             time.Duration
	LatencyP50             time.Duration // Median
	LatencyP95             time.Duration
	LatencyP99             time.Duration
	LatencyData            []time.Duration // Final complete latency data
	ErrorDetails           map[string]int  // Count of specific errors encountered
	Error                  error           // *** ADDED: Field for critical run error ***
}

// HttpStatusError represents a non-2xx HTTP response.
type HttpStatusError struct {
	StatusCode int
	Status     string
}

func (e *HttpStatusError) Error() string {
	return fmt.Sprintf("HTTP status error: %d %s", e.StatusCode, e.Status)
}

// --- Calculation Helpers ---

func CalculateAverage(data []time.Duration) time.Duration {
	if len(data) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range data {
		total += d
	}
	// Use float64 for division to avoid potential overflow with large sums/counts
	return time.Duration(float64(total) / float64(len(data)))
}

func CalculatePercentile(sortedData []time.Duration, percentile float64) time.Duration {
	n := len(sortedData)
	if n == 0 {
		return 0
	}
	if percentile <= 0 {
		return sortedData[0]
	}
	if percentile >= 100 {
		return sortedData[n-1]
	}

	// Calculate index using floating point (0-based) - Rank method
	index := (percentile / 100.0) * float64(n)

	// Use the value at the ceiling of the rank (adjusting for 0-based index)
	ceilIndex := int(math.Ceil(index)) - 1
	if ceilIndex >= n {
		ceilIndex = n - 1
	} // Boundary check
	if ceilIndex < 0 {
		ceilIndex = 0
	} // Boundary check

	return sortedData[ceilIndex]
}

// Helper to sort latency data (operates in-place)
func SortLatencies(data []time.Duration) {
	sort.Slice(data, func(i, j int) bool {
		return data[i] < data[j]
	})
}
