//go:build windows

package agentattach

import (
	"os"
)

func notifyResize(ch chan<- os.Signal) {
}

func stopResize(ch chan os.Signal) {
}
