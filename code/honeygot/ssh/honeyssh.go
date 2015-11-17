package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gombadi/honeygot/code/honeygot/batcher"
	"golang.org/x/crypto/ssh"
)

var extIP string // external ip that this system uses

type SSHServer struct {
	port   string // port to listen on
	socket net.Listener
}

func New() *SSHServer {
	return &SSHServer{port: "22022"}
}

func (s *SSHServer) Start() error {

	var err error
	if port := os.Getenv("HONEYGOT_SSHPORT"); port != "" {
		s.port = port
	}

	var extURL string
	if extURL = os.Getenv("HONEYGOT_EXTURL"); extURL == "" {
		extURL = "http://checkip.dyndns.org/"
	}

	fmt.Printf("extURL: %s\n", extURL)

	// get our external ip address so we can add it to the results
	resp, err := http.Get(extURL)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	extIP = strings.TrimSpace(string(body))

	// start the ssh server listening and return ?
	config := ssh.ServerConfig{
		PasswordCallback:  authPassword,
		PublicKeyCallback: authKey,
	}

	// generate a new private key each startcso it looks like a new server.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	privateKeyDer := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privateKeyDer,
	}

	hostPrivateKeySigner, err := ssh.ParsePrivateKey(pem.EncodeToMemory(&privateKeyBlock))
	if err != nil {
		return err
	}
	config.AddHostKey(hostPrivateKeySigner)

	s.socket, err = net.Listen("tcp", ":"+s.port)
	if err != nil {
		return err
	}

	go s.listenForConn(config)

	return nil
}

// Close will close the listening server and shutdown the system
func (s *SSHServer) Close() {
	s.socket.Close()
}

// listenForConn runs in a goroutine to listen for incoming connections
func (s *SSHServer) listenForConn(config ssh.ServerConfig) {

	dowhile := true
	for dowhile == true {
		conn, err := s.socket.Accept()
		if err != nil {
			dowhile = false
		} else {
			// handle each incoming request in its own goroutine
			go s.handleSSH(conn, config)
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

	batcher.AddToBatch(fmt.Sprintf("%d sshPass %s %s %s %s",
		time.Now().Unix(),
		strings.Split(conn.RemoteAddr().String(), ":")[0],
		extIP,
		conn.User(),
		strconv.QuoteToASCII(string(password))))

	return nil, errAuthenticationFailed
}

// authKey records any incoming request trying to auth with an ssh key
func authKey(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {

	h := sha256.New()
	h.Write(key.Marshal())
	sum := h.Sum(nil)

	batcher.AddToBatch(fmt.Sprintf("%d sshKey %s %s %s %s %s",
		time.Now().Unix(),
		strings.Split(conn.RemoteAddr().String(), ":")[0],
		extIP,
		conn.User(),
		key.Type(),
		base64.StdEncoding.EncodeToString(sum)))

	return nil, errAuthenticationFailed
}

/*

 */
