# Tasks

## 1. Render the launcher script per platform
- [x] 1.1 Split `externalTerminalScript` into a POSIX renderer and a Windows batch renderer,
      selected by target platform rather than by the compiling host, so both are testable anywhere.
- [x] 1.2 Give the Windows script `set`, `cd /d`, the command, and a self-delete so the agent
      token does not outlive the run.
- [x] 1.3 Keep rejecting environment variable names that are not valid identifiers.

## 2. Launch the host terminal correctly on Windows
- [x] 2.1 Write the temporary script with a `.cmd` extension on Windows and skip the POSIX chmod.
- [x] 2.2 Join `PATH` with the platform separator instead of a hardcoded `:`.
- [x] 2.3 Default to `wt.exe` when present, otherwise `cmd.exe`, keeping the existing
      `{script}` / `{cmd}` override.

## 3. Supervise a Windows run
- [x] 3.1 Implement `startControlledCommand` on a Job Object with a new process group.
- [x] 3.2 Implement `stopControlledCommand`: break the group when asked politely, terminate the
      job when forced.
- [x] 3.3 Quote the supervised command line for the host shell.

## 4. Route the execution
- [x] 4.1 Make `detectDefaultTerminal` report the host terminal on Windows instead of `pty`.
- [x] 4.2 Call `dispatchTerminal` from the dispatch and console paths and launch the host
      terminal when it resolves to anything but `pty`.
- [x] 4.3 Report the launch with the mode that was actually used.

## 5. Tests
- [x] 5.1 Windows and POSIX script rendering, including the rejected variable name.
- [x] 5.2 Quoting for both hosts.
- [x] 5.3 Routing: Windows resolves to a host terminal, other platforms keep the PTY.
- [x] 5.4 Skip the PTY-bound suites on platforms without a pseudo-terminal instead of failing.

## 6. Gates
- [x] 6.1 `go build ./...`, `go vet ./...`, `go test ./...` and the web gate.
- [x] 6.2 Cross-compile the agent for darwin and linux to prove the build tags hold.
