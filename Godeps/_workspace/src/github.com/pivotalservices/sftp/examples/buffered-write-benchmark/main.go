// buffered-write-benchmark benchmarks the peformance of writing
// a single large []byte on the client to /dev/null on the server via io.Copy.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"gopkg.in/datianshi/crypto.v1/ssh"
	"gopkg.in/datianshi/crypto.v1/ssh/agent"

	"github.com/pivotalservices/sftp"
)

var (
	USER = flag.String("user", os.Getenv("USER"), "ssh username")
	HOST = flag.String("host", "localhost", "ssh server hostname")
	PORT = flag.Int("port", 22, "ssh server port")
	PASS = flag.String("pass", os.Getenv("SOCKSIE_SSH_PASSWORD"), "ssh password")
)

func init() {
	flag.Parse()
}

func main() {
	var auths []ssh.AuthMethod
	if aconn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(aconn).Signers))

	}
	if *PASS != "" {
		auths = append(auths, ssh.Password(*PASS))
	}

	config := ssh.ClientConfig{
		User: *USER,
		Auth: auths,
	}
	addr := fmt.Sprintf("%s:%d", *HOST, *PORT)
	conn, err := ssh.Dial("tcp", addr, &config)
	if err != nil {
		log.Fatalf("unable to connect to [%s]: %v", addr, err)
	}
	defer conn.Close()

	c, err := sftp.NewClient(conn)
	if err != nil {
		log.Fatalf("unable to start sftp subsytem: %v", err)
	}
	defer c.Close()

	w, err := c.OpenFile("/dev/null", syscall.O_WRONLY)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	f, err := os.Open("/dev/zero")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	const size = 1e9

	log.Printf("writing %v bytes", size)
	t1 := time.Now()
	n, err := w.Write(make([]byte, size))
	if err != nil {
		log.Fatal(err)
	}
	if n != size {
		log.Fatalf("copy: expected %v bytes, got %d", size, n)
	}
	log.Printf("wrote %v bytes in %s", size, time.Since(t1))
}
