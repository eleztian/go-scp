package scp

import (
	"golang.org/x/crypto/ssh"
)

const defaultRemoteBinary = "/usr/bin/scp"

type SCP struct {
	remoteBinary string
	useSFTP      bool
	cli          *ssh.Client
}

type Option func(s *SCP)

func WithSFTP(enable bool) Option {
	return func(s *SCP) {
		s.useSFTP = enable
	}
}

func WithRemoteSShBinaryPath(path string) Option {
	return func(s *SCP) {
		s.remoteBinary = path
	}
}

func New(addr string, cfg *ssh.ClientConfig, ops ...Option) (*SCP, error) {
	cli, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	res := &SCP{remoteBinary: defaultRemoteBinary, cli: cli}

	for _, op := range ops {
		op(res)
	}
	return res, nil
}

func (s *SCP) Close() error {
	return s.cli.Close()
}

func (s *SCP) Upload(local string, remote string) error {
	ses, err := s.newSession()
	if err != nil {
		return err
	}
	defer ses.Close()

	return ses.Send(local, remote)
}

func (s *SCP) Download(remote string, local string) error {
	ses, err := s.newSession()
	if err != nil {
		return err
	}
	defer ses.Close()

	return ses.Recv(remote, local)
}

func (s *SCP) newSession() (Session, error) {
	if s.useSFTP {
		return NewSFTPSession(s.cli)
	} else {
		return NewScpSession(s.cli, s.remoteBinary)
	}
}
