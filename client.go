package ftp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Knows the control connection where commands are sent to the server.
type Connection struct {
	control  io.ReadWriteCloser
	hostname string
}

var CRLF = "\r\n"
var ASCII = "A"
var BINARY = "I"
var IMAGE = "I" //Synonymous with "Binary"

// Dials up a remote FTP server.
// host should be in the form of address:port e.g. myserver:21 or myserver:ftp
// Returns a pointer to a Connection
func Dial(host string) (*Connection, error) {
	if host == "" {
		return nil, fmt.Errorf("FTP Connection Error: Host can not be blank!")
	}
	if !hasPort(host) {
		return nil, fmt.Errorf("FTP Connection Error: Host must have a port! e.g. host:21")
	}
	conn, err := net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}
	// timeoutDuration := 5 * time.Second
	// conn.setReadDeadline(time.Now().Add(timeoutDuration))

	// Upon connect, most servers respond with a welcome message.
	// The welcome message contains a status code, just like any other command.
	// TODO: Handle servers with no welcome message.
	welcomeMsg := make([]byte, 1024)
	_, err = conn.Read(welcomeMsg)
	if err != nil {
		return nil, fmt.Errorf("Couldn't read the server's initital connection information. Error: %v", err)
	}
	code, err := strconv.Atoi(string(welcomeMsg[0:3]))
	err = checkResponseCode(2, uint(code))
	if err != nil {
		return nil, fmt.Errorf("Couldn't read the server's Welcome Message. Error: %v", err)
	}
	// This doesn't work for IPv6 addresses.
	hostParts := strings.Split(host, ":")
	// return &Connection{conn, hostParts[0], conn}, nil
	return &Connection{conn, hostParts[0]}, nil
}

// Executes an FTP command.
// Sends the command to the server.
// Returns the response code, the response text from the server, and any errors.
// The response code will be zero if an error is encountered.
// The response string will be the empty string if an error is encountered.
func (c *Connection) Cmd(command string, arg string) (code uint, response string, err error) {
	// Format command to be sent to the server.
	formattedCommand := command + " " + arg + CRLF

	// Send command to the server.
	_, err = c.control.Write([]byte(formattedCommand))
	if err != nil {
		return 0, "", err
	}

	// Process the response.
	reader := bufio.NewReader(c.control)
	regex := regexp.MustCompile("[0-9][0-9][0-9] ")
	for {
		ln, err := reader.ReadString('\n')
		if err != nil {
			return 0, "", err
		}

		response += ln
		if regex.MatchString(ln) {
			break
		}
	}
	t, err := strconv.Atoi(response[0:3])
	if err != nil {
		return 0, response, err
	}
	return uint(t), response, err
}

// Log into a FTP server using username and password.
func (c *Connection) Login(user string, password string) error {
	if user == "" {
		return fmt.Errorf("FTP Connection Error: User can not be blank!")
	}
	if password == "" {
		return fmt.Errorf("FTP Connection Error: Password can not be blank!")
	}
	// TODO: Check the server's response codes.
	_, _, err := c.Cmd("USER", user)
	_, _, err = c.Cmd("PASS", password)
	if err != nil {
		return err
	}
	return nil
}

func (c *Connection) Logout() error {
	_, _, err := c.Cmd("QUIT", "")
	if err != nil {
		return err
	}
	err = c.control.Close()
	if err != nil {
		return err
	}
	return nil
}

// Configure read deadline to timeout, wrapping
// net.Conn.setReadDeadline
// setReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func setReadDeadline(c net.Conn, duration uint) {
	// disable timeout used for test failure
	timeoutDuration := time.Duration(time.Duration(duration) * time.Second)
	c.SetReadDeadline(time.Now().Add(timeoutDuration))
}

// Download a file to a []byte slice and return it
func (c *Connection) GetBuffer(src, mode string, timeout uint) ([]byte, error) {
	// Use PASV to set up the data port.
	pasvCode, pasvLine, err := c.Cmd("PASV", "")
	if err != nil {
		return nil, err
	}
	pasvErr := checkResponseCode(2, pasvCode)
	if pasvErr != nil {
		msg := fmt.Sprintf("Cannot set PASV. Error: %v", pasvErr)
		return nil, fmt.Errorf(msg)
	}
	dataPort, err := extractDataPort(pasvLine)
	/*_, err = extractDataPort(pasvLine)*/
	if err != nil {
		return nil, err
	}

	// Set the TYPE (ASCII or Binary)
	typeCode, typeLine, err := c.Cmd("TYPE", mode)
	if err != nil {
		return nil, err
	}
	typeErr := checkResponseCode(2, typeCode)
	if typeErr != nil {
		msg := fmt.Sprintf("Cannot set TYPE. Error: '%v'. Line: '%v'", typeErr, typeLine)
		return nil, fmt.Errorf(msg)
	}

	// Can't use Cmd() for RETR because it doesn't return until *after* you've
	// downloaded the requested file.
	command := []byte("RETR " + src + CRLF)
	_, err = c.control.Write(command)
	if err != nil {
		return nil, err
	}

	// Open connection to remote data port.
	remoteConnectString := c.hostname + ":" + fmt.Sprintf("%d", dataPort)
	downloadConn, err := net.Dial("tcp", remoteConnectString)
	defer downloadConn.Close()
	if err != nil {
		msg := fmt.Sprintf("Couldn't connect to server's remote data port. Error: %v", err)
		return nil, fmt.Errorf(msg)
	}

	// Buffer for downloading and writing to file
	bufLen := 1024
	buf := make([]byte, bufLen)
	var result bytes.Buffer
	result.Grow(2 ^ (1024 * 1024))
	if timeout > 0 {
		setReadDeadline(downloadConn, timeout)
	}

	// Read from the server and write the contents to a file
	for {
		bytesRead, readErr := downloadConn.Read(buf)
		if bytesRead > 0 {
			for i, n := 0, 0; i < bytesRead; i += n {
				n, readErr = result.Write(buf[0:bytesRead])
				if err != nil {
					return nil, readErr
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return result.Bytes(), nil
}

// Alias for DownloadFile
func (c *Connection) Get(src, dest, mode string, timeout uint) error {
	return c.DownloadFile(src, dest, mode, timeout)
}

// Alias for UploadFile
func (c *Connection) Put(src, dest, mode string, timeout uint) error {
	return c.UploadFile(src, dest, mode, timeout)
}

// Download a file from a remote server.  Assumes only passive FTP
// connections for now. When timeout == 0, ignore, when timeout > 0
// set timeout limit
func (c *Connection) DownloadFile(src, dest, mode string, timeout uint) error {
	// Use PASV to set up the data port.
	pasvCode, pasvLine, err := c.Cmd("PASV", "")
	if err != nil {
		return err
	}
	pasvErr := checkResponseCode(2, pasvCode)
	if pasvErr != nil {
		msg := fmt.Sprintf("Cannot set PASV. Error: %v", pasvErr)
		return fmt.Errorf(msg)
	}
	dataPort, err := extractDataPort(pasvLine)
	/*_, err = extractDataPort(pasvLine)*/
	if err != nil {
		return err
	}

	// Set the TYPE (ASCII or Binary)
	typeCode, typeLine, err := c.Cmd("TYPE", mode)
	if err != nil {
		return err
	}
	typeErr := checkResponseCode(2, typeCode)
	if typeErr != nil {
		msg := fmt.Sprintf("Cannot set TYPE. Error: '%v'. Line: '%v'", typeErr, typeLine)
		return fmt.Errorf(msg)
	}

	// Can't use Cmd() for RETR because it doesn't return until *after* you've
	// downloaded the requested file.
	command := []byte("RETR " + src + CRLF)
	_, err = c.control.Write(command)
	if err != nil {
		return err
	}

	// Open connection to remote data port.
	remoteConnectString := c.hostname + ":" + fmt.Sprintf("%d", dataPort)
	downloadConn, err := net.Dial("tcp", remoteConnectString)
	defer downloadConn.Close()
	if err != nil {
		msg := fmt.Sprintf("Couldn't connect to server's remote data port. Error: %v", err)
		return fmt.Errorf(msg)
	}

	// Set up the destination file
	var filePerms = os.FileMode(0664)
	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY, filePerms)
	defer destFile.Close()
	if err != nil {
		msg := fmt.Sprintf("Cannot open destination file, '%s'. %v", dest, err)
		return fmt.Errorf(msg)
	}

	// Buffer for downloading and writing to file
	bufLen := 1024
	buf := make([]byte, bufLen)

	if timeout > 0 {
		setReadDeadline(downloadConn, timeout)
	}

	// Read from the server and write the contents to a file
	for {
		bytesRead, readErr := downloadConn.Read(buf)
		if bytesRead > 0 {
			_, err := destFile.Write(buf[0:bytesRead])
			if err != nil {
				msg := fmt.Sprintf("Coudn't write to file, '%s'. Error: %v", dest, err)
				return fmt.Errorf(msg)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// Put a file on the ftp server in the location specified by dest. When
// timeout == 0, ignore, when timeout > 0 set timeout limit
func (c *Connection) UploadFile(src, dest, mode string, timeout uint) error {
	// Use PASV to set up the data port.
	pasvCode, pasvLine, err := c.Cmd("PASV", "")
	if err != nil {
		return err
	}
	pasvErr := checkResponseCode(2, pasvCode)
	if pasvErr != nil {
		msg := fmt.Sprintf("Cannot set PASV. Error: %v", pasvErr)
		return fmt.Errorf(msg)
	}
	dataPort, err := extractDataPort(pasvLine)
	if err != nil {
		return err
	}

	// Set the TYPE (ASCII or Binary)
	typeCode, typeLine, err := c.Cmd("TYPE", mode)
	if err != nil {
		return err
	}
	typeErr := checkResponseCode(2, typeCode)
	if typeErr != nil {
		msg := fmt.Sprintf("Cannot set TYPE. Error: '%v'. Line: '%v'", typeErr, typeLine)
		return fmt.Errorf(msg)
	}
	// Can't use Cmd() for STOR because it doesn't return until *after* you've
	// uploaded the requested file.
	command := []byte("STOR " + dest + CRLF)
	_, err = c.control.Write(command)
	if err != nil {
		return err
	}

	// Open connection to remote data port.
	remoteConnectString := c.hostname + ":" + fmt.Sprintf("%d", dataPort)
	uploadConn, err := net.Dial("tcp", remoteConnectString)
	defer uploadConn.Close()
	if err != nil {
		msg := fmt.Sprintf("Couldn't connect to server's remote data port. Error: %v", err)
		return fmt.Errorf(msg)
	}

	// Open the source file for uploading
	sourceFile, err := os.OpenFile(src, os.O_RDONLY, 0644)
	defer sourceFile.Close()
	if err != nil {
		msg := fmt.Sprintf("Cannot open src file, '%s'. %v", src, err)
		return fmt.Errorf(msg)
	}

	// Buffer for uploading the file
	bufLen := 1024
	buf := make([]byte, bufLen)

	if timeout > 0 {
		setReadDeadline(uploadConn, timeout)
	}

	// Read from the file and write the contents to the server
	for {
		bytesRead, readErr := sourceFile.Read(buf)
		if bytesRead > 0 {
			_, writeErr := uploadConn.Write(buf[0:bytesRead])
			if writeErr != nil {
				msg := fmt.Sprintf("Couldn't write file to server, '%s'. Error: %v", sourceFile.Name(), writeErr)
				return fmt.Errorf(msg)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// Given an prefix, does the response code match the expected code?
func checkResponseCode(expectCode, code uint) error {
	if 1 <= expectCode && expectCode < 10 && code/100 != expectCode ||
		10 <= expectCode && expectCode < 100 && code/10 != expectCode ||
		100 <= expectCode && expectCode < 1000 && code != expectCode {
		msg := fmt.Sprintf("Bad response from server. Expected: %d, Got: %d", expectCode, code)
		return fmt.Errorf(msg)
	}
	return nil
}

// Interrogate a server response for the remote port on which to connect.
// Returns the port to be used as the data port for transfers.
func extractDataPort(line string) (port uint, err error) {
	// We only care about the last two octets
	portPattern := "[0-9]+,[0-9]+,[0-9]+,[0-9]+,([0-9]+,[0-9]+)"
	re, err := regexp.Compile(portPattern)
	if err != nil {
		return 0, err
	}
	match := re.FindStringSubmatch(line)
	if len(match) == 0 {
		msg := "Cannot find data port in server output: " + line
		return 0, fmt.Errorf(msg)
	}
	octets := strings.Split(match[1], ",")
	firstOctet, _ := strconv.Atoi(octets[0])
	secondOctet, _ := strconv.Atoi(octets[1])
	port = uint(firstOctet*256 + secondOctet)

	return port, nil
}

// Reused from src/pkg/http/client.go
// Given a string of the form "host", "host:port", or "[ipv6::address]:port",
// return true if the string includes a port.
func hasPort(s string) bool { return strings.LastIndex(s, ":") > strings.LastIndex(s, "]") }
