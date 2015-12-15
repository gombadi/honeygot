package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type MySQLServer struct {
	b      *batcher // batcher to handle the auth events produced
	port   string   // port to listen on
	socket net.Listener
}

func startMySQL(port string, b *batcher) (*MySQLServer, error) {

	m := &MySQLServer{
		port: port,
		b:    b,
	}

	var err error
	if port := os.Getenv("HONEYGOT_MYSQLPORT"); port != "" {
		m.port = port
	}

	// start the mysql server
	m.socket, err = net.Listen("tcp", ":"+m.port)
	if err != nil {
		return nil, err
	}

	go m.listenForConn()

	return m, nil
}

// Close will close the listening server and shutdown the system
func (m *MySQLServer) Close() {
	m.socket.Close()
}

// listenForConn runs in a goroutine to listen for incoming connections
func (m *MySQLServer) listenForConn() {

	for {
		conn, err := m.socket.Accept()
		if err == nil {
			// handle each incoming request in its own goroutine
			go m.handleConn(conn)
		} else {
			log.Printf("mysql server socket.Accept failed: %v - mysql server exiting", err)
			break
		}
	}
}

// handleConn runs in a goroutine and handles an incoming MySQL connection
func (m *MySQLServer) handleConn(c net.Conn) {

	// close the connection to the remote user
	defer c.Close()

	conn := newConn(c)

	err := conn.Handshake()

	r := &AuthEvent{
		Time:        fmt.Sprintf("%d", time.Now().Unix()),
		AuthType:    "mysqlPass",
		SrcIP:       conn.host,
		DestIP:      extIP,
		User:        conn.user,
		Credentials: conn.credentials,
		TypeData:    fmt.Sprintf("salt: 0x%x dbname: %s err: %v", conn.salt, conn.db, err),
	}

	r.updateHash()
	addToBatch(r)

	// FIXME - pull data from the conn and send to batcher
	//log.Printf("send to batcher - ip: %s user: %s credentials: %s salt: 0x%x db: %s err: %v\n", conn.host, conn.user, conn.credentials, conn.salt, conn.db, err)

}

type MyConn struct {
	pkg          *PacketIO
	c            net.Conn
	charset      string
	user         string
	credentials  string
	db           string
	host         string
	salt         []byte
	capability   uint32
	connectionId uint32
	status       uint16
	collation    uint8
}

func (c *MyConn) Handshake() error {
	if err := c.writeInitialHandshake(); err != nil {
		log.Printf("send initial handshake error %s", err.Error())
		return err
	}
	return c.readHandshakeResponse()
}

func newConn(co net.Conn) *MyConn {
	c := new(MyConn)
	c.c = co
	c.pkg = NewPacketIO(co)
	c.pkg.Sequence = 0
	c.connectionId = atomic.AddUint32(&baseConnId, 1)
	c.status = SERVER_STATUS_AUTOCOMMIT
	c.salt = RandomBuf(20)
	// fixed salt means we can see if same pw is used
	//c.salt = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
	c.collation = 33
	c.charset = "utf8"

	return c
}

func (c *MyConn) writeInitialHandshake() error {
	data := make([]byte, 4, 128)

	//min version 10
	data = append(data, 10)

	//server version[00]
	data = append(data, ServerVersion...)
	data = append(data, 0)

	//connection id
	data = append(data, byte(c.connectionId), byte(c.connectionId>>8), byte(c.connectionId>>16), byte(c.connectionId>>24))

	//auth-plugin-data-part-1
	data = append(data, c.salt[0:8]...)

	//filter [00]
	data = append(data, 0)

	// optional fields from here
	//capability flag lower 2 bytes, using default capability here
	data = append(data, byte(DEFAULT_CAPABILITY), byte(DEFAULT_CAPABILITY>>8))

	//charset, utf-8 default - 33
	data = append(data, uint8(33))

	//status
	data = append(data, byte(c.status), byte(c.status>>8))

	//below 13 byte may not be used
	//capability flag upper 2 bytes, using default capability here
	data = append(data, byte(DEFAULT_CAPABILITY>>16), byte(DEFAULT_CAPABILITY>>24))

	//filter [0x15], for wireshark dump, value is 0x15
	data = append(data, 0x15)

	//reserved 10 [00]
	data = append(data, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)

	//auth-plugin-data-part-2
	data = append(data, c.salt[8:]...)

	//filter [00]
	data = append(data, 0)

	return c.writePacket(data)
}

func RandomBuf(size int) []byte {
	buf := make([]byte, size)
	rand.Seed(time.Now().UTC().UnixNano())
	for i := 0; i < size; i++ {
		buf[i] = byte(rand.Intn(127))
		if buf[i] == 0 || buf[i] == byte('$') {
			buf[i]++
		}
	}
	return buf
}
func (c *MyConn) readPacket() ([]byte, error) {
	return c.pkg.ReadPacket()
}

func (c *MyConn) writePacket(data []byte) error {
	return c.pkg.WritePacket(data)
}

// readHandshakeResponse reads the client response and extracts the interesting info.
// This will always return an error
func (c *MyConn) readHandshakeResponse() error {
	data, err := c.readPacket()

	if err != nil {
		return err
	}

	//pos := 0

	//capability
	c.capability = binary.LittleEndian.Uint32(data[:4])

	//skip unused parts
	pos := 32

	//user name
	c.user = string(data[pos : pos+bytes.IndexByte(data[pos:], 0)])
	pos += len(c.user) + 1

	//auth length and auth
	authLen := int(data[pos])
	pos++

	c.credentials = base64.StdEncoding.EncodeToString(data[pos : pos+authLen])

	c.host, _, _ = net.SplitHostPort(c.c.RemoteAddr().String())

	pos += authLen

	if c.capability&CLIENT_CONNECT_WITH_DB > 0 {
		if len(data[pos:]) != 0 {
			c.db = string(data[pos : pos+bytes.IndexByte(data[pos:], 0)])
		}
	}

	// generate the error to be returned and logged
	authErr := c.genErrMessage()

	// always return an error to the remote client
	err = c.writeError(authErr)
	if err != nil {
		return err
	} else {
		return authErr
	}
}

func (c *MyConn) writeError(e *SqlError) error {

	data := make([]byte, 4, 16+len(e.Message))

	data = append(data, ERR_HEADER)
	data = append(data, byte(e.Code), byte(e.Code>>8))

	if c.capability&CLIENT_PROTOCOL_41 > 0 {
		data = append(data, '#')
		data = append(data, e.State...)
	}

	data = append(data, e.Message...)

	return c.writePacket(data)
}

func (c *MyConn) genErrMessage() *SqlError {

	// if it is the user we have configured then return a different error message
	if strings.Contains("drupal", c.user) {
		if checkPW(c.credentials, []byte("drupalpw"), c.salt) {
			return &SqlError{
				Code:    ER_TOO_MANY_USER_CONNECTIONS,
				State:   ER_TOO_MANY_USER_CONNECTIONS_STATE,
				Message: fmt.Sprintf("User %-.64s already has more than 'max_user_connections' active connections", c.user),
				Desc:    "Too many connections"}
		}
	}

	usePW := "YES"
	if len(c.credentials) == 0 {
		usePW = "NO"
	}

	return &SqlError{
		Code:    ER_ACCESS_DENIED_ERROR,
		State:   ER_ACCESS_DENIED_STATE,
		Message: fmt.Sprintf("Access denied for user '%-.48s'@'%-.64s' (using password: %s)", c.user, c.host, usePW),
		Desc:    "Bad Credentials"}
}

// checkPW will compare the local password with the supplied remote password
func checkPW(remotepw string, localpw, salt []byte) bool {
	if len(localpw) == 0 {
		return false
	}

	// stage1Hash = SHA1(password)
	crypt := sha1.New()
	crypt.Write(localpw)
	stage1 := crypt.Sum(nil)

	// scrambleHash = SHA1(scramble + SHA1(stage1Hash))
	// inner Hash
	crypt.Reset()
	crypt.Write(stage1)
	hash := crypt.Sum(nil)

	// outer Hash
	crypt.Reset()
	crypt.Write(salt)
	crypt.Write(hash)
	p1 := crypt.Sum(nil)

	// token = scrambleHash XOR stage1Hash
	for i := range p1 {
		p1[i] ^= stage1[i]
	}
	return strings.EqualFold(base64.StdEncoding.EncodeToString(p1), remotepw)
}

//=========================================================
type SqlError struct {
	Message string
	State   string
	Desc    string
	Code    uint16
}

func (e *SqlError) Error() string {
	//return fmt.Sprintf("ERROR %d (%s): %s", e.Code, e.State, e.Message)
	return e.Desc
}

//=========================================================

var (
	ErrBadConn = errors.New("connection was bad")

	baseConnId uint32 = 17362

	DEFAULT_CAPABILITY uint32 = CLIENT_LONG_PASSWORD | CLIENT_LONG_FLAG |
		CLIENT_CONNECT_WITH_DB | CLIENT_PROTOCOL_41 |
		CLIENT_TRANSACTIONS | CLIENT_SECURE_CONNECTION
)

const (
	ER_ERROR_FIRST               uint16 = 1000
	ER_ACCESS_DENIED_ERROR       uint16 = 1045
	ER_TOO_MANY_USER_CONNECTIONS uint16 = 1203
	SERVER_STATUS_AUTOCOMMIT     uint16 = 0x0002
)

const (
	MinProtocolVersion                 byte   = 10
	MaxPayloadLen                      int    = 1<<24 - 1
	TimeFormat                         string = "2006-01-02 15:04:05"
	ServerVersion                      string = "5.5.46-0+deb8u1"
	AUTH_NAME                          string = "mysql_native_password"
	ER_ACCESS_DENIED_STATE             string = "28000"
	ER_TOO_MANY_USER_CONNECTIONS_STATE string = "42000"
)

const (
	OK_HEADER          byte = 0x00
	ERR_HEADER         byte = 0xff
	EOF_HEADER         byte = 0xfe
	LocalInFile_HEADER byte = 0xfb
)

const (
	CLIENT_LONG_PASSWORD uint32 = 1 << iota
	CLIENT_FOUND_ROWS
	CLIENT_LONG_FLAG
	CLIENT_CONNECT_WITH_DB
	CLIENT_NO_SCHEMA
	CLIENT_COMPRESS
	CLIENT_ODBC
	CLIENT_LOCAL_FILES
	CLIENT_IGNORE_SPACE
	CLIENT_PROTOCOL_41
	CLIENT_INTERACTIVE
	CLIENT_SSL
	CLIENT_IGNORE_SIGPIPE
	CLIENT_TRANSACTIONS
	CLIENT_RESERVED
	CLIENT_SECURE_CONNECTION
	CLIENT_MULTI_STATEMENTS
	CLIENT_MULTI_RESULTS
	CLIENT_PS_MULTI_RESULTS
	CLIENT_PLUGIN_AUTH
	CLIENT_CONNECT_ATTRS
	CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA
)

/*
var errAuthenticationFailed = errors.New("Invalid credentials. Please try again")

// authPassword records any incoming request trying to auth with a username/password
func authPassword(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {

	r := &AuthEvent{
		Time:        fmt.Sprintf("%d", time.Now().Unix()),
		AuthType:    "sshPass",
		SrcIP:       strings.Split(conn.RemoteAddr().String(), ":")[0],
		DestIP:      extIP,
		User:        conn.User(),
		Credentials: strconv.QuoteToASCII(string(password)),
	}

	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%ssshPass%s%s%s%s", r.Time, r.SrcIP, r.DestIP, r.User, r.Credentials)))
	r.Hash = base64.StdEncoding.EncodeToString(h.Sum(nil))

	addToBatch(r)

	return nil, errAuthenticationFailed
}
*/

/*

 */
