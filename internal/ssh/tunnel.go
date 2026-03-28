package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/maestro-go/maestro/core/conf"
	"golang.org/x/crypto/ssh"
)

// Tunnel represents an SSH tunnel.
type Tunnel struct {
	config     *conf.SSHConfig
	client     *ssh.Client
	listener   net.Listener
	localPort  uint16
	remoteAddr string
}

// NewTunnel creates a new SSH tunnel.
func NewTunnel(sshConfig *conf.SSHConfig, remoteHost string, remotePort uint16, localPort uint16) *Tunnel {
	return &Tunnel{
		config:     sshConfig,
		localPort:  localPort,
		remoteAddr: fmt.Sprintf("%s:%d", remoteHost, remotePort),
	}
}

// Start establishes the SSH connection and starts forwarding traffic.
func (t *Tunnel) Start(ctx context.Context) error {
	var authMethods []ssh.AuthMethod

	if t.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(t.config.Password))
	}

	if t.config.KeyPath != "" {
		key, err := os.ReadFile(t.config.KeyPath)
		if err != nil {
			return fmt.Errorf("failed to read private key: %w", err)
		}

		var signer ssh.Signer
		if t.config.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(t.config.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	sshConfig := &ssh.ClientConfig{
		User:            t.config.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", t.config.Host, t.config.Port), sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial SSH: %w", err)
	}
	t.client = client

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", t.localPort))
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to listen on local port: %w", err)
	}
	t.listener = listener

	go func() {
		for {
			localConn, err := listener.Accept()
			if err != nil {
				return
			}

			go t.forward(localConn)
		}
	}()

	return nil
}

func (t *Tunnel) forward(localConn net.Conn) {
	defer localConn.Close()

	remoteConn, err := t.client.Dial("tcp", t.remoteAddr)
	if err != nil {
		return
	}
	defer remoteConn.Close()

	copyConn := func(writer, reader net.Conn) {
		_, _ = io.Copy(writer, reader)
	}

	go copyConn(localConn, remoteConn)
	copyConn(remoteConn, localConn)
}

// Close closes the SSH tunnel.
func (t *Tunnel) Close() error {
	if t.listener != nil {
		t.listener.Close()
	}
	if t.client != nil {
		return t.client.Close()
	}
	return nil
}

// LocalPort returns the local port the tunnel is listening on.
func (t *Tunnel) LocalPort() uint16 {
	return t.localPort
}
