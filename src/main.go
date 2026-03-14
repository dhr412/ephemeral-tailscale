package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tailscale.com/tsnet"
)

type Config struct {
	AuthKey     string
	HostAddress string
	LocalPort   string
}

func readConfig() (Config, error) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("Tailscale auth key: ")
	scanner.Scan()
	authKey := strings.TrimSpace(scanner.Text())

	fmt.Print("Host tailnet address: ")
	scanner.Scan()
	hostAddr := strings.TrimSpace(scanner.Text())

	fmt.Print("Local proxy port (default 22122): ")
	scanner.Scan()
	localPort := strings.TrimSpace(scanner.Text())
	if localPort == "" {
		localPort = "22122"
	}
	port, err := strconv.Atoi(localPort)
	if err != nil {
		return Config{}, fmt.Errorf("invalid port number: %s", localPort)
	}
	if port < 1024 || port > 65535 {
		return Config{}, fmt.Errorf("port must be between 1024-65535 (user ports)")
	}

	if authKey == "" || hostAddr == "" {
		return Config{}, fmt.Errorf("auth key and host address are required")
	}

	return Config{
		AuthKey:     authKey,
		HostAddress: hostAddr,
		LocalPort:   localPort,
	}, nil
}

func run(ctx context.Context, config Config) error {
	tempDir, err := os.MkdirTemp("", "tssh-ephemeral-state-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	srv := &tsnet.Server{
		Hostname:  "tssh-ephemeral-node",
		AuthKey:   config.AuthKey,
		Ephemeral: true,
		Dir:       tempDir,
	}
	defer cleanup(srv, tempDir)

	fmt.Println("Connecting to tailnet...")
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start tsnet: %w", err)
	}

	time.Sleep(3 * time.Second)

	lc, err := srv.LocalClient()
	if err != nil {
		return fmt.Errorf("failed to get local client: %w", err)
	}

	status, err := lc.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	fmt.Printf("Connected to tailnet as %s\n", status.Self.HostName)

	listener, err := net.Listen("tcp", "127.0.0.1:"+config.LocalPort)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	defer listener.Close()

	fmt.Printf("\nProxy running on localhost:%s\n", config.LocalPort)
	fmt.Printf("Connect with: ssh <user>@localhost -p %s\n\n", config.LocalPort)

	errChan := make(chan error, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					errChan <- err
					return
				}
			}
			go handleConnection(ctx, srv, conn, config.HostAddress)
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errChan:
		return err
	}
}

func handleConnection(ctx context.Context, srv *tsnet.Server, clientConn net.Conn, hostAddr string) {
	defer clientConn.Close()

	hostConn, err := srv.Dial(ctx, "tcp", hostAddr+":22")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to dial host: %v\n", err)
		return
	}
	defer hostConn.Close()

	done := make(chan struct{}, 2)

	go func() {
		io.Copy(hostConn, clientConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(clientConn, hostConn)
		done <- struct{}{}
	}()

	<-done
}

func cleanup(srv *tsnet.Server, tempDir string) {
	fmt.Println("\nCleaning up...")
	srv.Close()
	if tempDir != "" {
		os.RemoveAll(tempDir)
	}
	fmt.Println("Cleanup complete")
}

func printUsage() {
	fmt.Print("A portable tool for establishing temporary SSH access to remote hosts over a private Tailscale network\n\n")
	fmt.Printf("Usage: %s\n", os.Args[0])
	fmt.Println("Commands:\n\t--help, help\t\tShow this message")
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-help", "-h", "help":
			printUsage()
			return
		}
	}

	config, err := readConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	if err := run(ctx, config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
