package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHServer struct {
	b      *batcher // batcher to handle the auth events produced
	port   string   // port to listen on
	socket net.Listener
}

func startSSH(port string, b *batcher) (*SSHServer, error) {

	var err error

	s := &SSHServer{
		port: port,
		b:    b,
	}

	// start the ssh server listening
	config := ssh.ServerConfig{
		PasswordCallback:  authPassword,
		PublicKeyCallback: authKey,
		ServerVersion:     "SSH-2.0-OpenSSH_6.7p1 Debian-5",
	}

	// generate a new private key each startcso it looks like a new server.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	privateKeyDer := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privateKeyDer,
	}

	hostPrivateKeySigner, err := ssh.ParsePrivateKey(pem.EncodeToMemory(&privateKeyBlock))
	if err != nil {
		return nil, err
	}
	config.AddHostKey(hostPrivateKeySigner)

	s.socket, err = net.Listen("tcp", ":"+s.port)
	if err != nil {
		return nil, err
	}

	go s.listenForConn(config)

	return s, nil
}

// Close will close the listening server and shutdown the system
func (s *SSHServer) Close() {
	s.socket.Close()
}

// listenForConn runs in a goroutine to listen for incoming connections
func (s *SSHServer) listenForConn(config ssh.ServerConfig) {

	for {
		conn, err := s.socket.Accept()
		if err == nil {
			// handle each incoming request in its own goroutine
			go s.handleSSH(conn, config)
		} else {
			log.Printf("ssh server socket.Accept failed: %v - ssh server exiting\n", err)
			break
		}
	}
}

// handleSSH runs in a goroutine and handles an incoming SSH connection
func (s *SSHServer) handleSSH(conn net.Conn, config ssh.ServerConfig) {

	_, _, _, err := ssh.NewServerConn(conn, &config)
	if err == nil {
		// this should never happen and if it does we need to shutdown the system
		log.Fatal("ssh server error: successful login. Shutting down system\n")
	}
}

var errAuthenticationFailed = errors.New("Invalid credentials. Please try again")

// authPassword records any incoming request trying to auth with a username/password
func authPassword(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {

	r := &AuthEvent{
		Time:        fmt.Sprintf("%d", time.Now().Unix()),
		AuthType:    "sshPass",
		SrcIP:       strings.Split(conn.RemoteAddr().String(), ":")[0],
		DestIP:      extIP,
		User:        conn.User(),
		TypeData:    fmt.Sprintf("client-version: %s", strconv.QuoteToASCII(string(conn.ClientVersion()))),
		Credentials: strconv.QuoteToASCII(string(password)),
	}

	addToBatch(r)

	return nil, errAuthenticationFailed
}

// authKey records any incoming request trying to auth with an ssh key
func authKey(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {

	r := &AuthEvent{
		Time:     fmt.Sprintf("%d", time.Now().Unix()),
		AuthType: "sshKey",
		SrcIP:    strings.Split(conn.RemoteAddr().String(), ":")[0],
		DestIP:   extIP,
		User:     conn.User(),
		TypeData: fmt.Sprintf("ssh-key-type: %s client-version: %s", key.Type(), strconv.QuoteToASCII(string(conn.ClientVersion()))),
	}

	h := sha256.New()
	h.Write(key.Marshal())
	r.Credentials = base64.StdEncoding.EncodeToString(h.Sum(nil))

	addToBatch(r)

	return nil, errAuthenticationFailed
}

/*

 */
