# go-scp

使用go-scp非常容易实现在两个host之间copy文件/文件夹.
go-scp基于golang.org/x/crypto/ssh包和remote host建立一个安全的连接，通过SCP协议复制文件.

## Example

```golang
package main

import (
	"golang.org/x/crypto/ssh"
	"net"
)

var (
	addr     = "192.168.0.102:22"
	user     = "root"
	password = "password"
)

func main() {

	cfg := &ssh.ClientConfig{
		Config: ssh.Config{},
		User:   user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	scp, err := New(addr, cfg)
	checkErr(err)
	defer scp.Close()

	err = scp.Upload("testdata", "/root/scp")
	checkErr(err)
	err = scp.Download("/root/scp", "testdata")
	checkErr(err)
	return
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
```