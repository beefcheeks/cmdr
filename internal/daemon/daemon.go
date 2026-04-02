package daemon

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/mikehu/cmdr/internal/scheduler"
)

const (
	sockName = "cmdr.sock"
	pidFile  = "cmdr.pid"
)

func runtimeDir() string {
	dir := filepath.Join(os.TempDir(), "cmdr")
	os.MkdirAll(dir, 0o700)
	return dir
}

func sockPath() string {
	return filepath.Join(runtimeDir(), sockName)
}

func pidPath() string {
	return filepath.Join(runtimeDir(), pidFile)
}

// Run starts the daemon in the foreground (blocking).
func Run() error {
	if err := writePID(); err != nil {
		return fmt.Errorf("writing pid: %w", err)
	}
	defer cleanup()

	s := scheduler.New()
	s.Start()
	defer s.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"running","pid":%d,"tasks":%d}`, os.Getpid(), len(s.Tasks()))
	})
	mux.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		task := r.URL.Query().Get("task")
		if task == "" {
			http.Error(w, "missing ?task= parameter", http.StatusBadRequest)
			return
		}
		if err := s.RunTask(task); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		fmt.Fprintf(w, `{"ran":"%s"}`, task)
	})

	// Listen on unix socket so CLI can talk to daemon
	os.Remove(sockPath())
	ln, err := net.Listen("unix", sockPath())
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	srv := &http.Server{Handler: mux}

	// Graceful shutdown on SIGTERM/SIGINT
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sig
		fmt.Println("\ncmdr: shutting down")
		srv.Close()
	}()

	fmt.Printf("cmdr: daemon running (pid %d)\n", os.Getpid())
	if err := srv.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Start launches the daemon as a background process.
func Start() error {
	if pid, running := isRunning(); running {
		return fmt.Errorf("cmdr is already running (pid %d)", pid)
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	proc, err := os.StartProcess(exe, []string{exe, "start", "-f"}, &os.ProcAttr{
		Dir:   "/",
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	if err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}
	proc.Release()
	fmt.Println("cmdr: daemon started")
	return nil
}

// Stop sends SIGTERM to the running daemon.
func Stop() error {
	pid, running := isRunning()
	if !running {
		return fmt.Errorf("cmdr is not running")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("sending signal: %w", err)
	}
	fmt.Println("cmdr: stop signal sent")
	return nil
}

// Status prints daemon status.
func Status() error {
	pid, running := isRunning()
	if !running {
		fmt.Println("cmdr: not running")
		return nil
	}
	fmt.Printf("cmdr: running (pid %d)\n", pid)

	// Try querying the daemon's HTTP endpoint
	client := &http.Client{
		Transport: unixDialer(sockPath()),
	}
	resp, err := client.Get("http://cmdr/status")
	if err == nil {
		defer resp.Body.Close()
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		fmt.Println(string(body[:n]))
	}
	return nil
}

func writePID() error {
	return os.WriteFile(pidPath(), []byte(strconv.Itoa(os.Getpid())), 0o644)
}

func cleanup() {
	os.Remove(pidPath())
	os.Remove(sockPath())
}

func isRunning() (int, bool) {
	data, err := os.ReadFile(pidPath())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}
	// Signal 0 checks if process exists
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return 0, false
	}
	return pid, true
}
