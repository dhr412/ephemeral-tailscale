package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

type ClientConfig struct {
	AuthKey     string
	HostAddress string
	SSHUsername string
	SSHPort     string
}

func readClientConfig() ClientConfig {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("Tailscale auth key: ")
	scanner.Scan()
	authKey := strings.TrimSpace(scanner.Text())

	fmt.Print("Host tailnet address: ")
	scanner.Scan()
	hostAddr := strings.TrimSpace(scanner.Text())

	fmt.Print("SSH username: ")
	scanner.Scan()
	username := strings.TrimSpace(scanner.Text())

	fmt.Print("SSH port (default 22): ")
	scanner.Scan()
	sshPort := strings.TrimSpace(scanner.Text())
	if sshPort == "" {
		sshPort = "22"
	}

	return ClientConfig{
		AuthKey:     authKey,
		HostAddress: hostAddr,
		SSHUsername: username,
		SSHPort:     sshPort,
	}
}

func detectDistro() (string, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", err
	}

	content := string(data)
	if strings.Contains(content, "ID=ubuntu") || strings.Contains(content, "ID=debian") {
		return "apt", nil
	}
	if strings.Contains(content, "ID=arch") {
		return "pacman", nil
	}
	if strings.Contains(content, "ID=fedora") || strings.Contains(content, "ID=rhel") {
		return "dnf", nil
	}

	return "", fmt.Errorf("unsupported distribution")
}

func runPrivileged(ignore bool, name string, args ...string) error {
	if _, err := exec.LookPath("sudo"); err == nil {
		cmd := exec.Command("sudo", append([]string{name}, args...)...)
		if !ignore {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
		}
		return cmd.Run()
	}

	suCmd := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	cmd := exec.Command("su", "-c", suCmd)
	if !ignore {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
	}
	return cmd.Run()
}

func ensureTailscale() error {
	if _, err := exec.LookPath("tailscale"); err == nil {
		return nil
	}

	fmt.Println("Tailscale not found, installing...")

	distro, err := detectDistro()
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch distro {
	case "apt":
		cmd = exec.Command("sh", "-c", "curl -fsSL https://tailscale.com/install.sh | sh")
	case "pacman":
		return runPrivileged(false, "pacman", "-S", "--noconfirm", "tailscale")
	case "dnf":
		return runPrivileged(false, "dnf", "install", "-y", "tailscale")
	default:
		return fmt.Errorf("no installation method for distro")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startTailscaled() (int, error) {
	_ = runPrivileged(true, "systemctl", "stop", "tailscaled")

	fmt.Println("Starting tailscaled in ephemeral mode...")

	pidFile := "/tmp/tailscaled-ephemeral.pid"
	cmdStr := fmt.Sprintf(`nohup tailscaled --state=mem: --encrypt-state >/dev/null 2>&1 & echo $! > %s`, pidFile)

	if err := runPrivileged(false, "sh", "-c", cmdStr); err != nil {
		return 0, fmt.Errorf("failed to start tailscaled: %w", err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, fmt.Errorf("could not read PID file: %w", err)
	}

	var pid int
	fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
	if pid == 0 {
		return 0, fmt.Errorf("could not retrieve tailscaled PID")
	}
	return pid, nil
}

func connectTailnet(authKey string) error {
	return runPrivileged(false, "tailscale", "up", "--authkey="+authKey, "--ephemeral")
}

func waitForConnection() error {
	cmd := exec.Command("tailscale", "status", "--wait")
	return cmd.Run()
}

func startSSH(config ClientConfig) error {
	args := []string{
		fmt.Sprintf("%s@%s", config.SSHUsername, config.HostAddress),
		"-p", config.SSHPort,
	}

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func cleanupClient(tailscaledPID int) {
	fmt.Println("\nCleaning up...")
	runPrivileged(false, "tailscale", "logout")
	if tailscaledPID != 0 {
		runPrivileged(false, "kill", fmt.Sprintf("%d", tailscaledPID))
	}
	runPrivileged(false, "rm", "-f", "/tmp/tailscaled-ephemeral.pid")
	fmt.Println("Cleanup complete")
}

func runCliented() {
	config := readClientConfig()

	var tailscaledPID int
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		if tailscaledPID != 0 {
			cleanupClient(tailscaledPID)
		}
		os.Exit(0)
	}()

	if err := ensureTailscale(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ensure Tailscale: %v\n", err)
		os.Exit(1)
	}

	pid, err := startTailscaled()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start tailscaled: %v\n", err)
		os.Exit(1)
	}
	tailscaledPID = pid

	if err := connectTailnet(config.AuthKey); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to tailnet: %v\n", err)
		os.Exit(1)
	}

	if err := waitForConnection(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to establish connection: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Connected to tailnet successfully")

	if err := startSSH(config); err != nil {
		fmt.Fprintf(os.Stderr, "SSH session failed: %v\n", err)
		os.Exit(1)
	}

	cleanupClient(tailscaledPID)
}
