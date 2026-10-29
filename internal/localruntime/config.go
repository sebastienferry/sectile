// Package localruntime defines the companion's public transport contract.
package localruntime

import (
	"fmt"
	"os"
)

const Port = 52963

const Origin = "http://127.0.0.1:52963"

type Metadata struct {
	Protocol       int               `json:"protocol"`
	Port           int               `json:"port"`
	Origin         string            `json:"origin"`
	OAuthCallbacks map[string]string `json:"oauthCallbacks"`
}

func Info() Metadata {
	return Metadata{Protocol: 1, Port: Port, Origin: Origin, OAuthCallbacks: map[string]string{
		"jira":   Origin + "/auth/jira/callback",
		"github": Origin + "/auth/github/callback",
		"gitlab": Origin + "/auth/gitlab/callback",
	}}
}

func Active() bool { return os.Getenv("SECTILE_LOCAL_RUNTIME_KEY") != "" }

// Callback leaves ordinary web/Electron deployments' registered URLs alone.
func Callback(provider string) string {
	if !Active() {
		return ""
	}
	return Info().OAuthCallbacks[provider]
}

// Validate refuses a changed callback origin rather than silently picking a port.
func Validate(host, port, dataDir string) error {
	if host != "127.0.0.1" || port != fmt.Sprint(Port) || dataDir == "" {
		return fmt.Errorf("local runtime requires 127.0.0.1:%d and a private server data directory", Port)
	}
	return nil
}
