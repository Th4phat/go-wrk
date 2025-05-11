# Go-WRK: HTTP Load Testing Tool

Go-WRK is a flexible HTTP load testing tool written in Go, inspired by `wrk` and `wrk2`, designed for ease of use and extensibility. It features a terminal user interface (TUI) for interactive configuration and live monitoring of benchmarks.

![Screenshot of Go-WRK TUI (./res/screenshot3.png)]

## Features

*   **Interactive TUI:** Configure benchmarks, view live metrics, and manage test collections directly in your terminal.
*   **Configurable Parameters:**
    *   Target URL
    *   HTTP Method (GET, POST, PUT, DELETE, etc.)
    *   Number of concurrent threads (workers)
    *   Total number of persistent HTTP connections
    *   Benchmark duration
    *   Request payload (for methods like POST, PUT, PATCH)
*   **Live Metrics Display:**
    *   Requests Attempted/Completed
    *   Errors & Error Rate
    *   Throughput (Requests/Second)
    *   Latency Percentiles (Avg, P50, P95, P99)
    *   Live Latency Distribution Histogram
*   **Test Collections:**
    *   Save and load benchmark configurations from JSON files.
    *   Organize tests into named collections (directories).
    *   Easily re-run saved test scenarios.
*   **HTTP/1.1 & HTTP/2 Support:**
    *   Utilizes `fasthttp` for high-performance HTTP/1.1 requests.
    *   Supports for HTTP/2 isn't implemented yet.
*   **Cross-Platform:** Runs on Windows, macOS, and Linux.
*   **Debug Logging:** Optional detailed logging to a file for troubleshooting.

## Installation

### Prerequisites

*   **Go:** Version 1.18 or higher.

### From Source

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/Th4phat/go-wrk
    cd go-wrk
    ```

2.  **Build the executable:**
    ```bash
    go build -o go-wrk .
    ```
    This will create an executable named `go-wrk` (or `go-wrk.exe` on Windows) in the current directory.

3.  **Run:**
    You can then run it directly:
    ```bash
    ./go-wrk
    ```
    Or, move the executable to a directory in your system's `PATH` (e.g., `/usr/local/bin` or `~/bin` ) for easier access.

### Using `go install` (Recommended for users)

If you have Go installed and your `GOPATH/bin` (or `GOBIN`) is in your system's `PATH`:
```bash
go install https://github.com/Th4phat/go-wrk
```
This will download, build, and install the `go-wrk` executable into your Go binary directory.

## Usage

Run `go-wrk` from your terminal:

```bash
./go-wrk
```
or if installed via `go install`:
```bash
go-wrk
```

### Terminal User Interface (TUI)

Upon starting, you will be presented with the TUI.

**Navigation:**

*   **↑ / ↓ :** Navigate lists (collections, tests, methods).
*   **Enter:** Select an item, confirm input, or start a benchmark (when in the configuration view).
*   **Esc:** Go back to the previous view or cancel an input.
*   **q / Ctrl+C:** Quit the application.
*   **Ctrl+R:** Refresh the UI / Reset to the initial collections view.
*   **Ctrl+S:** (When in the configuration/Idle view) Save the current benchmark configuration as a new test.
*   **Ctrl+X:** (When a benchmark is running) Stop the current benchmark.
*   **?:** Toggle the help view showing all key bindings.

### Test Configuration Files

*   Tests are stored as JSON files in the ` $HOME/.config/gowrk` for linux and `%AppData%/Roaming/gowrk` directory (created automatically if it doesn't exist).
*   Each `.json` file within a collection directory represents a "Test".


### Debug Logging

To enable debug logging to a file (`debug.log` in the current directory), set the `BENCH_DEBUG` environment variable:

```bash
BENCH_DEBUG=true ./go-wrk
```
This log contains detailed TUI update cycles and internal engine messages, useful for troubleshooting.

## Advanced Configuration (Optional)

*(This section can be expanded later if you add features like custom headers, timeouts per request, etc., configurable via JSON or TUI)*

*   **HTTP Client Timeouts:** The `fasthttp.HostClient` has default read/write timeouts. These are currently hardcoded in `benchmark/engine.go` but could be made configurable.
*   **Custom Headers:** Currently, only `Content-Type: application/json` is set for payload methods. Future versions could allow custom headers via the config file or TUI.

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for bugs, feature requests, or improvements.

## License

This project is licensed under the [MIT License](LICENSE). (Create a LICENSE file with the MIT license text if you choose this one).
