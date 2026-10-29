//go:build !windows

package agentexec

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// StartOwned starts a provider behind a separate lifetime supervisor. Only the
// owning agent holds the pipe's write end: even SIGKILL closes it. The supervisor
// retains the provider as a child while it stops tools in separate sessions.
// The executable must dispatch the provider-guard role to RunProviderGuard.
func StartOwned(cmd *exec.Cmd) (func(), error) {
	binary, err := os.Executable()
	if err != nil {
		return func() {}, err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return func() {}, err
	}
	value, _ := ownedInputs.LoadOrStore(cmd, &ownedInputState{})
	state := value.(*ownedInputState)
	state.mu.Lock()
	state.owner = writer
	if state.closed {
		_, _ = writer.Write([]byte{'E'})
	}
	state.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			state.mu.Lock()
			_ = writer.Close()
			state.owner = nil
			state.mu.Unlock()
			ownedInputs.Delete(cmd)
		})
	}
	if cmd.Cancel != nil {
		cmd.Cancel = func() error { release(); return nil }
	}
	// Sending stdin EOF explicitly from the live owner distinguishes an ordinary
	// input close from the same pipe closing because the owner was SIGKILLed.
	input := cmd.Stdin
	if input == nil {
		input = strings.NewReader("")
	}
	if _, file := input.(*os.File); !file {
		cmd.Stdin = &ownedInput{Reader: input, state: state}
	}
	path, args := cmd.Path, append([]string(nil), cmd.Args[1:]...)
	fd := 3 + len(cmd.ExtraFiles)
	cmd.Path = binary
	cmd.Args = append([]string{binary, "provider-guard", strconv.Itoa(fd), path}, args...)
	cmd.ExtraFiles = append(cmd.ExtraFiles, reader)
	_, err = StartDetached(cmd)
	_ = reader.Close()
	if err != nil {
		release()
		return func() {}, err
	}
	return release, nil
}

// RunProviderGuard runs before the daemon's startup. Stdio is passed through
// without interpreting provider frames, credentials or prompts. The lifetime FD
// is deliberately not inherited by the provider or any of its tools.
func RunProviderGuard(args []string) int {
	if len(args) < 2 {
		return 2
	}
	fd, err := strconv.Atoi(args[0])
	if err != nil || fd < 3 {
		return 2
	}
	owner := os.NewFile(uintptr(fd), "provider-owner")
	if owner == nil {
		return 2
	}
	defer owner.Close()
	ownerGone := make(chan struct{})
	inputClosed := make(chan struct{})
	go func() {
		defer close(ownerGone)
		var value [1]byte
		notified := false
		for {
			n, err := owner.Read(value[:])
			if n > 0 && value[0] == 'E' && !notified {
				close(inputClosed)
				notified = true
			}
			if err != nil {
				return
			}
		}
	}()
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	if _, err := exec.LookPath("ps"); err != nil {
		fmt.Fprintln(os.Stderr, "Provider supervisor requires process-tree inspection:", err)
		return 1
	}
	cmd := Hidden(exec.Command(args[1], args[2:]...))
	// Do not forward owner EOF into the provider before tree cleanup: a CLI
	// that exits on stdin EOF could orphan a tool before we inspect descendants.
	input, writer, err := os.Pipe()
	if err != nil {
		return 1
	}
	defer input.Close()
	defer writer.Close()
	go func() {
		_, _ = io.Copy(writer, os.Stdin)
		select {
		case <-inputClosed:
			_ = writer.Close()
		case <-ownerGone:
		}
	}()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, os.Stdout, os.Stderr
	release, err := StartDetached(cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Provider supervisor could not start the CLI:", err)
		return 1
	}
	defer release()
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case err = <-waited:
		if err == nil {
			return 0
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		return 1
	case <-ownerGone:
	case <-signals:
	}
	// Never substitute a process-group signal: a provider tool can own another
	// session. Keep the root alive until descendant discovery has frozen its tree.
	reported := false
	for {
		if err := StopTree(cmd); err == nil {
			<-waited
			return 0
		} else if !reported {
			fmt.Fprintln(os.Stderr, "Provider supervisor is retrying tool termination:", err)
			reported = true
		}
		// Keep the ancestor alive on an inspection failure. Killing only its group
		// here would orphan descendants in separate sessions and prevent a retry.
		select {
		case <-waited:
			return 1
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// OwnedStdinPipe preserves exec.Cmd's file ownership while reporting an explicit
// input close to the guard. Wrapping the read file would make exec.Start close
// it before its copy goroutine can forward the provider's first message.
func OwnedStdinPipe(cmd *exec.Cmd) (io.WriteCloser, error) {
	writer, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	state := &ownedInputState{}
	ownedInputs.Store(cmd, state)
	return &ownedStdinWriter{WriteCloser: writer, state: state, cmd: cmd}, nil
}

var ownedInputs sync.Map

type ownedInputState struct {
	mu     sync.Mutex
	owner  *os.File
	closed bool
}

func (s *ownedInputState) inputClosed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.owner != nil {
		_, _ = s.owner.Write([]byte{'E'})
	}
}

type ownedStdinWriter struct {
	io.WriteCloser
	state *ownedInputState
	cmd   *exec.Cmd
}

func (w *ownedStdinWriter) Close() error {
	w.state.inputClosed()
	// If startup was refused before StartOwned, there is no guard to release.
	w.state.mu.Lock()
	notStarted := w.state.owner == nil
	w.state.mu.Unlock()
	if notStarted {
		ownedInputs.Delete(w.cmd)
	}
	return w.WriteCloser.Close()
}

type ownedInput struct {
	io.Reader
	state *ownedInputState
}

func (r *ownedInput) Read(data []byte) (int, error) {
	n, err := r.Reader.Read(data)
	if err != nil {
		r.state.inputClosed()
	}
	return n, err
}
