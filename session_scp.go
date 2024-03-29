package scp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
)

type scpSession struct {
	remoteBinary string
	*ssh.Session
	stdIn  io.WriteCloser
	stdOut io.Reader
}

func NewScpSession(sshCli *ssh.Client, remoteScpBinary string) (Session, error) {
	res := &scpSession{
		remoteBinary: remoteScpBinary,
	}
	ses, err := sshCli.NewSession()
	if err != nil {
		return nil, err
	}
	res.Session = ses

	res.stdIn, err = res.StdinPipe()
	if err != nil {
		return nil, err
	}
	res.stdOut, err = res.Session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *scpSession) Close() {
	_ = s.Session.Close()
}

func (s *scpSession) Send(local string, remote string) error {
	info, err := os.Stat(local)
	if err != nil {
		return err
	}

	wg, _ := errgroup.WithContext(context.TODO())
	wg.Go(func() error {
		defer s.stdIn.Close()

		rsp, err := ReadResp(s.stdOut)
		if err != nil {
			return err
		}
		if rsp.IsFailure() {
			return errors.New(rsp.GetMessage().String())
		}

		if info.IsDir() {
			return s.sendDir(local, remote)
		} else {
			return s.sendFile(local, remote)
		}
	})
	var rE error
	wg.Go(func() error {
		rE = s.Run(fmt.Sprintf("%s -rt %s", s.remoteBinary, filepath.Dir(remote)))
		return nil
	})

	if err = wg.Wait(); err != nil {
		return err
	}
	return rE
}

func (s *scpSession) sendFile(local string, remote string) error {
	_, remoteName := filepath.Split(remote)
	info, err := os.Stat(local)
	if err != nil {
		return err
	}

	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()

	err = NewFile(info.Mode(), remoteName, info.Size()).WriteStream(s.stdIn, f)
	if err != nil {
		return err
	}

	rsp, err := ReadResp(s.stdOut)
	if err != nil {
		return err
	}
	if rsp.IsFailure() {
		return errors.New(rsp.GetMessage().String())
	}

	//fmt.Printf("FILE: %s => %s %d\n", local, remote, info.Size())

	return err
}

func (s *scpSession) sendDir(local string, remotePath string) error {
	info, err := os.Stat(local)
	if err != nil {
		return err
	}

	err = NewDirBegin(info.Mode(), info.Name()).Write(s.stdIn)
	if err != nil {
		return err
	}
	rsp, err := ReadResp(s.stdOut)
	if err != nil {
		return err
	}
	if rsp.IsFailure() {
		return errors.New(rsp.GetMessage().String())
	}

	fs, err := ioutil.ReadDir(local)
	if err != nil {
		return err
	}
	for _, f := range fs {
		src, remote := filepath.Join(local, f.Name()), filepath.Join(remotePath, f.Name())

		if f.IsDir() {
			err = s.sendDir(src, remote)
		} else {
			err = s.sendFile(src, remote)
		}
		if err != nil {
			return err
		}
	}

	err = NewDirEnd().Write(s.stdIn)
	if err != nil {
		return err
	}

	rsp, err = ReadResp(s.stdOut)
	if err != nil {
		return err
	}
	if rsp.IsFailure() {
		return errors.New(rsp.GetMessage().String())
	}

	return err
}

func (s *scpSession) Recv(remote string, local string, handler FileHandler) error {
	wg, _ := errgroup.WithContext(context.TODO())
	wg.Go(func() error {
		defer s.stdIn.Close()

		err := NewOkRsp().Write(s.stdIn)
		if err != nil {
			return err
		}

		err = s.recvCmd(local, remote, true, handler)
		if err != nil && err != io.EOF {
			return err
		}
		return nil
	})
	var rE error
	wg.Go(func() error {
		rE = s.Run(fmt.Sprintf("%s -rf %s", s.remoteBinary, remote))
		return nil
	})

	if err := wg.Wait(); err != nil {
		return err
	}
	return rE
}

func (s *scpSession) recvCmd(local string, remote string, isFirst bool, handler FileHandler) error {
	rsp, err := ReadResp(s.stdOut)
	if err != nil {
		return err
	}

	if rsp.IsFailure() {
		return errors.New(rsp.GetMessage().String())
	}
	if rsp.IsDir() {
		mode, _, filename, err := rsp.GetMessage().FileInfo()
		if err != nil {
			return err
		}
		if isFirst {
			return s.recvDir(mode, local, filepath.Join(remote, filename), handler)
		} else {
			return s.recvDir(mode, filepath.Join(local, filename), filepath.Join(remote, filename), handler)
		}
	} else if rsp.IsFile() {
		mode, size, filename, err := rsp.GetMessage().FileInfo()
		if err != nil {
			return err
		}
		return s.recvFile(mode, size, filepath.Join(local, filename), handler)
	} else if rsp.IsEndDir() {
		return io.EOF
	} else {
		return errors.New("invalid protocol")
	}

}

func (s *scpSession) recvDir(mode os.FileMode, local string, remote string, handler FileHandler) error {
	err := os.MkdirAll(local, mode)
	if err != nil {
		_ = NewErrorRsp(err.Error()).Write(s.stdIn)
		return err
	}
	err = NewOkRsp().Write(s.stdIn)
	if err != nil {
		return err
	}

	for {
		err = s.recvCmd(local, remote, false, handler)
		if err != nil {
			if err == io.EOF { // dir end
				err = NewOkRsp().Write(s.stdIn)
				if err != nil {
					return err
				}
			}
			return err
		}
	}
}

func (s *scpSession) recvFile(mode os.FileMode, size int64, local string, handler FileHandler) error {
	err := NewOkRsp().Write(s.stdIn)
	if err != nil {
		return err
	}

	err = handler(local, mode, io.LimitReader(s.stdOut, size))
	if err != nil {
		_ = NewErrorRsp(err.Error()).Write(s.stdIn)
		return err
	}

	rsp, err := ReadResp(s.stdOut)
	if err != nil {
		_ = NewErrorRsp(err.Error()).Write(s.stdIn)
		return err
	}
	if rsp.IsFailure() {
		return errors.New(rsp.GetMessage().String())
	}

	return NewOkRsp().Write(s.stdIn)
}
