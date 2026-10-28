//go:build !windows

package agent

import "syscall"

// Opening a substituted FIFO must not block; a substituted link is refused.
const agentProfileOpenFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK
