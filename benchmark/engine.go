package benchmark

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Th4phat/go-wrk/config"
	"github.com/Th4phat/go-wrk/metrics"

	"github.com/valyala/fasthttp"
)

type Engine struct {
	status Status

	stopSignal chan struct{}
	wgGlobal   sync.WaitGroup

	mu sync.RWMutex
}

type Status int

const (
	StatusIdle Status = iota
	StatusRunning
	StatusStopping
	StatusFinished
)

func NewEngine() *Engine {
	return &Engine{
		status: StatusIdle,
	}
}

func (e *Engine) Start(
	cfg config.BenchmarkConfig,
	progressChan chan<- metrics.ProgressUpdate,
	resultChan chan<- metrics.BenchmarkResult,
) error {
	e.mu.Lock()
	if e.status == StatusRunning || e.status == StatusStopping {
		e.mu.Unlock()
		return fmt.Errorf("benchmark is already running or stopping")
	}
	e.status = StatusRunning
	e.stopSignal = make(chan struct{})
	e.mu.Unlock()

	parsedURL, err := url.Parse(cfg.TargetURL)
	if err != nil {
		e.mu.Lock()
		e.status = StatusIdle
		e.mu.Unlock()
		close(progressChan)
		close(resultChan)
		return fmt.Errorf("invalid target URL for fasthttp client: %w", err)
	}
	isTLS := parsedURL.Scheme == "https"

	hostClient := &fasthttp.HostClient{
		Addr:     parsedURL.Host,
		Name:     "github.com/Th4phat/go-wrk-fasthttp-client",
		MaxConns: cfg.Connections,

		ReadTimeout:                   30 * time.Second,
		WriteTimeout:                  10 * time.Second,
		MaxIdleConnDuration:           90 * time.Second,
		IsTLS:                         isTLS,
		NoDefaultUserAgentHeader:      true,
		DisableHeaderNamesNormalizing: true,
		Dial: (&fasthttp.TCPDialer{
			Concurrency:      4096,
			DNSCacheDuration: time.Hour,
		}).Dial,
	}

	duration, err := time.ParseDuration(cfg.Duration)
	if err != nil {
		e.mu.Lock()
		e.status = StatusIdle
		e.mu.Unlock()
		close(progressChan)
		close(resultChan)
		return fmt.Errorf("invalid duration format in config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), duration)

	go func() {
		<-e.stopSignal
		cancel()
	}()

	e.wgGlobal.Add(1)
	go func() {
		defer e.wgGlobal.Done()
		defer cancel()
		defer close(progressChan)
		defer close(resultChan)

		var finalResult metrics.BenchmarkResult
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "[Engine] Panic recovered: %v\n%s\n", r, string(debug.Stack()))
				if finalResult.Config == nil {
					finalResult.Config = &cfg
				}
				finalResult.Error = fmt.Errorf("panic during benchmark: %v", r)
				select {
				case resultChan <- finalResult:
				case <-time.After(200 * time.Millisecond):
				}
			}
			e.mu.Lock()
			e.status = StatusFinished
			e.mu.Unlock()
		}()

		finalResult = e.runCollector(ctx, cfg, hostClient, progressChan)

		select {
		case resultChan <- finalResult:
		case <-ctx.Done():
			if finalResult.Error == nil {
				finalResult.Error = fmt.Errorf("benchmark interrupted before final result sent")
				select {
				case resultChan <- finalResult:
				default:
				}
			}
		case <-time.After(1 * time.Second):
			if finalResult.Error == nil {
				finalResult.Error = fmt.Errorf("timed out sending final result to TUI")
				select {
				case resultChan <- finalResult:
				default:
				}
			}
		}
	}()

	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.status == StatusRunning && e.stopSignal != nil {
		e.status = StatusStopping
		select {
		case <-e.stopSignal:
		default:
			close(e.stopSignal)
		}
	}
}

func (e *Engine) GetStatus() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

func (e *Engine) Wait() {
	e.wgGlobal.Wait()
}
