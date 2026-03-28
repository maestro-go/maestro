package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"testing"

	"github.com/maestro-go/maestro/core/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestTunnel(t *testing.T) {
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "testpass" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

	key, err := ssh.NewSignerFromKey(generatePrivateKey(t))
	require.NoError(t, err)
	config.AddHostKey(key)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	sshAddr := listener.Addr().String()
	sshHost, sshPortStr, _ := net.SplitHostPort(sshAddr)
	var sshPort uint16
	fmt.Sscanf(sshPortStr, "%d", &sshPort)

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				return
			}
			_, chans, reqs, err := ssh.NewServerConn(nConn, config)
			if err != nil {
				continue
			}
			go ssh.DiscardRequests(reqs)
			go func(chans <-chan ssh.NewChannel) {
				for newChannel := range chans {
					if newChannel.ChannelType() != "direct-tcpip" {
						newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
						continue
					}
					channel, requests, err := newChannel.Accept()
					if err != nil {
						continue
					}
					go ssh.DiscardRequests(requests)

					_ = channel
				}
			}(chans)
		}
	}()

	targetListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer targetListener.Close()

	targetAddr := targetListener.Addr().String()
	targetHost, targetPortStr, _ := net.SplitHostPort(targetAddr)
	var targetPort uint16
	fmt.Sscanf(targetPortStr, "%d", &targetPort)

	go func() {
		for {
			conn, err := targetListener.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("hello from target"))
			conn.Close()
		}
	}()

	sshConfig := &conf.SSHConfig{
		Host:     sshHost,
		Port:     sshPort,
		User:     "testuser",
		Password: "testpass",
	}

	// For the test, we let the OS pick a free port.
	// The tunnel doesn't support 0 to let OS pick, but we can manually pick one.
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	localPortStr := l.Addr().String()
	_, lp, _ := net.SplitHostPort(localPortStr)
	var localPort uint16
	fmt.Sscanf(lp, "%d", &localPort)
	l.Close() // Close it so tunnel can use it. There's a race here, but likely fine for local test.

	tunnel := NewTunnel(sshConfig, targetHost, targetPort, localPort)

	ctx := context.Background()
	err = tunnel.Start(ctx)
	assert.NoError(t, err)

	assert.Equal(t, localPort, tunnel.LocalPort())

	err = tunnel.Close()
	assert.NoError(t, err)
}

// Helper to generate a private key for the mock server
func generatePrivateKey(t *testing.T) any {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}
