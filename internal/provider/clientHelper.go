package provider

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getDefaultSocketPath() (string, error) {
	uid := os.Getuid()
	if uid == 0 {
		// Root user
		return "/run/podman/podman.sock", nil
	}
	// Non-root user
	return fmt.Sprintf("/run/user/%d/podman/podman.sock", uid), nil
}

func getPodmanConnectionURI(connectionName string) (string, error) {
	// Try to get connection info using podman system connection list
	cmd := exec.Command("podman", "system", "connection", "list", "--format", "{{.Name}} {{.URI}}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list podman connections: %w", err)
	}

	// Parse the output to find the matching connection
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 && parts[0] == connectionName {
			return parts[1], nil
		}
	}

	return "", fmt.Errorf("connection '%s' not found", connectionName)
}
