package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"tailscale.com/tsnet"
)

type EmbeddedProxyConfig struct {
	AuthKey     string
	HostAddress string
	LocalPort   string
}

func readEmbeddedProxyConfig() EmbeddedProxyConfig {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("Tailscale auth key: ")
	scanner.Scan()
	authKey := strings.TrimSpace(scanner.Text())

	fmt.Print("Host tailnet address: ")
	scanner.Scan()
	hostAddr := strings.TrimSpace(scanner.Text())

	fmt.Print("Local proxy port (default 2222): ")
	scanner.Scan()
	localPort := strings.TrimSpace(scanner.Text())
	if localPort == "" {
		localPort = "2222"
	}

	return EmbeddedProxyConfig{
		AuthKey:     authKey,
		HostAddress: hostAddr,
		LocalPort:   localPort,
	}
}

func runEmbeddedProxy(ctx context.Context, config EmbeddedProxyConfig) error {
	srv := &tsnet.Server{
		Hostname:  "ephemeral-ssh-proxy",
		AuthKey:   config.AuthKey,
		Ephemeral: true,
		Dir:       "/tmp/tsnet-ssh-proxy",
	}
	defer cleanupEmbeddedProxy(srv)

	fmt.Println("Connecting to tailnet...")
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start tsnet: %w", err)
	}

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

func cleanupEmbeddedProxy(srv *tsnet.Server) {
	fmt.Println("\nCleaning up...")
	srv.Close()
	os.RemoveAll("/tmp/tsnet-ssh-proxy")
	fmt.Println("Cleanup complete")
}

func runEmbedded() {
	config := readEmbeddedProxyConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	if err := runEmbeddedProxy(ctx, config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
