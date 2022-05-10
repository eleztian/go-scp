package scp

import (
	"net"
	"os"
	"testing"

	"golang.org/x/crypto/ssh"
)

var RemoteADDR = os.Getenv("TEST_SSH_ADDR")
var Password = os.Getenv("TEST_SSH_PASSWORD")
var RemotePath = os.Getenv("TEST_SSH_PATH")

func TestSCP_Download(t *testing.T) {
	scp, err := New(RemoteADDR, &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{ssh.Password(Password)},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}, WithSFTP(true))
	if err != nil {
		t.Error(err)
		return
	}
	defer scp.Close()
	err = scp.Download(RemotePath, "test")
	if err != nil {
		t.Error(err)
		return
	}
}

func TestSCP_Upload(t *testing.T) {
	scp, err := New(RemoteADDR, &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{ssh.Password(Password)},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}, WithSFTP(true))
	if err != nil {
		t.Error(err)
		return
	}
	defer scp.Close()
	err = scp.Upload("test", RemotePath)
	if err != nil {
		t.Error(err)
		return
	}
}
