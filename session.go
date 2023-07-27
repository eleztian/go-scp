package scp

import (
	"io"
	"os"
)

type Session interface {
	Close()
	Send(local string, remote string) error
	Recv(remote string, local string, handler FileHandler) error
}

type FileHandler func(path string, mode os.FileMode, reader io.Reader) error

func localFileHandler(path string, mode os.FileMode, reader io.Reader) error {
	dstF, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, mode)
	if err != nil {
		return err
	}
	defer dstF.Close()

	_, err = io.Copy(dstF, reader)
	if err != nil {
		return err
	}
	return nil
}
