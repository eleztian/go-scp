package scp

type Session interface {
	Close()
	Send(local string, remote string) error
	Recv(remote string, local string) error
}
