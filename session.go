package scp

import "io"

type Session interface {
	Close()
	Send(local string, remote string) error
	Recv(remote string, local string) error
	RecvStream(remote string, local io.Writer) error
}
