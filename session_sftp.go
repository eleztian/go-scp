package scp

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type sftpSession struct {
	cli *sftp.Client
}

func NewSFTPSession(sshCli *ssh.Client) (Session, error) {
	sftpCli, err := sftp.NewClient(sshCli)
	if err != nil {
		return nil, err
	}
	res := &sftpSession{
		cli: sftpCli,
	}
	return res, err
}

func (s *sftpSession) Close() {
	_ = s.cli.Close()
}

func (s *sftpSession) Send(local string, remote string) (err error) {
	local, err = filepath.Abs(local)
	if err != nil {
		return err
	}
	files := make([][2]string, 0)
	err = filepath.WalkDir(local, func(path string, d fs.DirEntry, err error) error {
		r, err := filepath.Rel(local, path)
		if err != nil {
			return err
		}
		files = append(files, [2]string{path, s.cli.Join(remote, r)})
		return nil
	})
	if err != nil {
		return err
	}
	for _, paire := range files {
		p, err := os.Readlink(paire[0])
		if err != nil {
			info, err := os.Stat(paire[0])
			if err != nil {
				return err
			}
			if info.IsDir() {
				err = s.cli.MkdirAll(paire[1])
				if err != nil && !os.IsExist(err) {
					return err
				}
			} else {
				err = s.copyFileToRemote(paire[1], paire[0], info.Mode())
				if err != nil {
					return err
				}
			}
		} else {
			if filepath.IsAbs(p) {
				rel, _ := filepath.Rel(filepath.Dir(paire[0]), p)
				_ = s.cli.Remove(paire[1])
				err = s.cli.Symlink(rel, paire[1])
				if err != nil {
					return err
				}
			} else {
				_ = s.cli.Remove(paire[1])
				err = s.cli.Symlink(p, paire[1])
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (s *sftpSession) Recv(remote string, local string, handler FileHandler) error {
	p, err := s.cli.ReadLink(remote)
	if err != nil {
		info, err := s.cli.Stat(remote)
		if err != nil {
			return err
		}
		if info.IsDir() {
			err = os.MkdirAll(local, info.Mode())
			if err != nil && !os.IsExist(err) {
				return err
			}

			files, err := s.cli.ReadDir(remote)
			if err != nil {
				return err
			}

			for _, f := range files {
				name := f.Name()
				err = s.Recv(s.cli.Join(remote, name), filepath.Join(local, name), handler)
				if err != nil {
					return err
				}
			}
		} else {
			err = s.copyFileFromRemote(remote, local, info.Mode(), handler)
			if err != nil {
				return err
			}
		}
	} else {
		if filepath.IsAbs(p) {
			rel, _ := filepath.Rel(filepath.Dir(remote), p)
			_ = os.Symlink(rel, local)
		} else {
			_ = os.Symlink(p, local)
		}
	}

	return nil
}

func (s *sftpSession) copyFileStreamFromRemote(remote string, local io.Writer) error {
	srcF, err := s.cli.Open(remote)
	if err != nil {
		return err
	}

	defer srcF.Close()

	_, err = io.Copy(local, srcF)
	if err != nil {
		return err
	}
	return nil
}

func (s *sftpSession) copyFileFromRemote(remote string, local string, mode os.FileMode, handler FileHandler) error {
	srcF, err := s.cli.Open(remote)
	if err != nil {
		return err
	}

	defer srcF.Close()
	n, _ := os.Stat(local)
	if n != nil && n.IsDir() {
		_, f := filepath.Split(remote)
		local = filepath.Join(local, f)
	}

	return handler(local, mode, srcF)
}

func (s *sftpSession) copyFileToRemote(remote string, local string, mode os.FileMode) error {
	srcF, err := os.Open(local)
	if err != nil {
		return err
	}
	defer srcF.Close()

	defer s.cli.Chmod(remote, mode)

	dstF, err := s.cli.OpenFile(remote, os.O_CREATE|os.O_TRUNC|os.O_RDWR)
	if err != nil {
		return err
	}
	defer dstF.Close()

	_, err = io.Copy(dstF, srcF)
	if err != nil {
		return err
	}

	return nil
}
