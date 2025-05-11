package benchmark

import (
	"context"
	"errors"
	"fmt"

	// "io" // No longer needed for fasthttp response body directly
	// "os" // For debug prints if any
	"strings"
	"sync"
	"time"

	"github.com/Th4phat/go-wrk/config"
	"github.com/Th4phat/go-wrk/metrics"

	"github.com/valyala/fasthttp" // Import fasthttp
)

func runWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	workerID int,
	hostClient *fasthttp.HostClient,
	cfg config.BenchmarkConfig,
	resultsChan chan<- time.Duration,
	errorsChan chan<- error,
) {
	defer wg.Done()

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(cfg.TargetURL)
	req.Header.SetMethod(cfg.Method)

	var payloadBytes []byte
	if (cfg.Method == "POST" || cfg.Method == "PUT" || cfg.Method == "PATCH") && cfg.Payload != "" {
		req.Header.SetContentType("application/json")
		payloadBytes = []byte(cfg.Payload)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if payloadBytes != nil {
			req.SetBody(payloadBytes)
		}

		reqStartTime := time.Now()
		err := hostClient.Do(req, resp)
		latency := time.Since(reqStartTime)

		if err != nil {
			if !(errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context canceled")) {
				select {
				case <-ctx.Done():
				case errorsChan <- err:
				}
			}
			continue
		}

		statusCode := resp.StatusCode()
		if statusCode >= 200 && statusCode < 300 {
			select {
			case resultsChan <- latency:
			case <-ctx.Done():
			}
		} else {
			httpErr := &metrics.HttpStatusError{
				StatusCode: statusCode,
				Status:     string(resp.Header.StatusMessage()),
			}
			select {
			case errorsChan <- httpErr:
			case <-ctx.Done():
			}
		}
	}
}

func (e *Engine) runCollector(
	ctx context.Context,
	cfg config.BenchmarkConfig,
	hostClient *fasthttp.HostClient,
	progressChan chan<- metrics.ProgressUpdate,
) metrics.BenchmarkResult {
	startTime := time.Now()

	bufferFactor := 2
	cfg.Threads = cfg.Threads * 20
	if cfg.Threads > cfg.Connections && cfg.Connections > 0 {
		bufferFactor = (cfg.Threads / cfg.Connections) * 2
		if bufferFactor < 2 {
			bufferFactor = 2
		}
	} else if cfg.Connections == 0 && cfg.Threads > 0 {
		bufferFactor = cfg.Threads * 2
	}
	resultsChanBufferSize := cfg.Connections * bufferFactor
	if resultsChanBufferSize < 100 {
		resultsChanBufferSize = 100
	}
	if resultsChanBufferSize > 10000 && cfg.Threads < 1000 {
		resultsChanBufferSize = 10000
	}

	resultsChan := make(chan time.Duration, resultsChanBufferSize)
	errorsChan := make(chan error, resultsChanBufferSize)

	var wgWorkers sync.WaitGroup
	workersDoneChan := make(chan struct{})

	wgWorkers.Add(cfg.Threads)
	for i := 0; i < cfg.Threads; i++ {
		go runWorker(ctx, &wgWorkers, i, hostClient, cfg, resultsChan, errorsChan)
	}

	go func() {
		wgWorkers.Wait()
		close(workersDoneChan)
	}()

	requestsCompleted := 0
	errorCount := 0
	var latencyData []time.Duration
	errorDetails := make(map[string]int)

	progressTicker := time.NewTicker(1 * time.Second)
	defer progressTicker.Stop()
	contextAlreadyDone := false

collectorLoop:
	for {
		select {
		case <-ctx.Done():
			if !contextAlreadyDone {
				contextAlreadyDone = true
				progressTicker.Stop()
			}

		case latency, ok := <-resultsChan:
			if !ok {
				resultsChan = nil
				if errorsChan == nil && workersDoneChan == nil {
					break collectorLoop
				}
				continue
			}
			requestsCompleted++
			latencyData = append(latencyData, latency)

		case err, ok := <-errorsChan:
			if !ok {
				errorsChan = nil
				if resultsChan == nil && workersDoneChan == nil {
					break collectorLoop
				}
				continue
			}
			errorCount++
			// ... (error key generation as before) ...
			errKey := "Unknown Error"
			if errors.Is(err, fasthttp.ErrTimeout) {
				errKey = "Timeout Error (fasthttp)"
			} else if errors.Is(err, fasthttp.ErrConnectionClosed) {
				errKey = "Connection Closed (fasthttp)"
			} else if errors.Is(err, fasthttp.ErrNoFreeConns) {
				errKey = "No Free Connections (fasthttp)"
			} else if errors.Is(err, fasthttp.ErrPipelineOverflow) {
				errKey = "Pipeline Overflow (fasthttp)"
			} else if httpErr, ok := err.(*metrics.HttpStatusError); ok {
				errKey = fmt.Sprintf("HTTP %d", httpErr.StatusCode)
			} else if err != nil {
				errStr := err.Error()
				if errors.Is(err, context.Canceled) {
					errKey = "Context Canceled"
				} else if errors.Is(err, context.DeadlineExceeded) {
					errKey = "Context Deadline Exceeded"
				} else if strings.Contains(errStr, "connection refused") {
					errKey = "Connection Refused"
				} else if strings.Contains(errStr, "no such host") {
					errKey = "DNS Error"
				} else {
					errKey = "Network Error"
				}
			}
			errorDetails[errKey]++

		case <-progressTicker.C:
			if !contextAlreadyDone {
				now := time.Now()
				elapsed := now.Sub(startTime)
				currentAttempted := requestsCompleted + errorCount
				var currentThroughput float64
				if elapsed.Seconds() > 0.01 {
					currentThroughput = float64(requestsCompleted) / elapsed.Seconds()
				}
				var currentErrorRate float64
				if currentAttempted > 0 {
					currentErrorRate = float64(errorCount) / float64(currentAttempted) * 100
				}
				var progressLatencySample []time.Duration
				const maxProgressSample = 1000
				if len(latencyData) > maxProgressSample {
					progressLatencySample = make([]time.Duration, maxProgressSample)
					copy(progressLatencySample, latencyData[len(latencyData)-maxProgressSample:])
				} else {
					progressLatencySample = make([]time.Duration, len(latencyData))
					copy(progressLatencySample, latencyData)
				}
				metrics.SortLatencies(progressLatencySample)
				latencyAvg := metrics.CalculateAverage(progressLatencySample)
				latencyP95 := metrics.CalculatePercentile(progressLatencySample, 95)
				latencyP99 := metrics.CalculatePercentile(progressLatencySample, 99)
				progressMsg := metrics.ProgressUpdate{
					Timestamp: now, RequestsAttempted: currentAttempted, RequestsCompleted: requestsCompleted, Errors: errorCount,
					CurrentThroughput: currentThroughput, CurrentErrorRate: currentErrorRate,
					LatencyAvg: latencyAvg, LatencyP95: latencyP95, LatencyP99: latencyP99,
					LatencyData: progressLatencySample,
				}
				select {
				case progressChan <- progressMsg:
				default:
				}
			}

		case <-workersDoneChan:
			workersDoneChan = nil
			break collectorLoop
		}
	}
	close(resultsChan)
	close(errorsChan)

	for latency := range resultsChan {
		requestsCompleted++
		latencyData = append(latencyData, latency)
	}

	for _ = range errorsChan {
		errorCount++
		errorDetails["Drained Error (Final Loop)"]++
	}
	endTime := time.Now()
	totalDuration := endTime.Sub(startTime)
	finalAttempted := requestsCompleted + errorCount
	if totalDuration < 1*time.Millisecond {
		totalDuration = 1 * time.Millisecond
	}
	finalThroughput := float64(requestsCompleted) / totalDuration.Seconds()
	var finalErrorRate float64
	if finalAttempted > 0 {
		finalErrorRate = float64(errorCount) / float64(finalAttempted) * 100
	}
	metrics.SortLatencies(latencyData)
	finalLatencyAvg := metrics.CalculateAverage(latencyData)
	finalLatencyP50 := metrics.CalculatePercentile(latencyData, 50)
	finalLatencyP95 := metrics.CalculatePercentile(latencyData, 95)
	finalLatencyP99 := metrics.CalculatePercentile(latencyData, 99)
	var finalError error
	if ctx.Err() == context.Canceled {
		finalError = fmt.Errorf("benchmark stopped by user")
	} else if ctx.Err() == context.DeadlineExceeded {
		if errorCount > 0 {
			finalError = fmt.Errorf("benchmark completed with %d errors", errorCount)
		}
	} else if errorCount > 0 {
		finalError = fmt.Errorf("benchmark finished with %d errors (unknown reason for stop)", errorCount)
	}

	finalResult := metrics.BenchmarkResult{
		Config: &cfg, TotalRequestsSent: finalAttempted, TotalRequestsCompleted: requestsCompleted, TotalErrors: errorCount,
		TotalDuration: totalDuration, Throughput: finalThroughput, ErrorRate: finalErrorRate,
		LatencyAvg: finalLatencyAvg, LatencyP50: finalLatencyP50, LatencyP95: finalLatencyP95, LatencyP99: finalLatencyP99,
		LatencyData: latencyData, ErrorDetails: errorDetails, Error: finalError,
	}
	return finalResult
}
