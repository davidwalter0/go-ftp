package ftp

import (
  /*"fmt"*/
  "io"
  "net"
  "os"
  "strconv"
  "strings"
)

// Knows the control connection where commands are sent to the server.
type Connection struct {
  control io.ReadWriteCloser
}

var CRLF = "\r\n"

// Executes an FTP command.
// Sends the command to the server.
// Returns the response code, and the response text from the server. 
func (c *Connection) Cmd(command string, arg string) (code uint, line string, err os.Error) {
  // Format command to be sent to the server.
  formatted_command := command + " " + arg + CRLF
  // TODO How big should this buffer be?
  var buf = make([]byte, 1024)
  // Send command to the server.
  _, err = c.control.Write([]byte(formatted_command))
  if err != nil {
    return 0, "", err
  }
  // Read the server's response.
  _, err = c.control.Read(buf)
  if err != nil {
    return 0, "", err
  }
  line = string(buf)
  code, err = strconv.Atoui(line[0:3])
  if err != nil {
    return 0, line, err
  }
  return code, line, err
}

// Log into a FTP server using username and password.
func (c *Connection) Login(user string, password string) os.Error {
  if user == "" {
    return os.NewError("FTP Connection Error: User can not be blank!")
  }
  if password == "" {
    return os.NewError("FTP Connection Error: Password can not be blank!")
  }
  // TODO: Check the server's response codes.
  _, _, err := c.Cmd("USER", user)
  _, _, err = c.Cmd("PASS", password)
  if err != nil {
    return err
  }
  return nil
}

// Dials up a remote FTP server.
// host should be in the form of address:port e.g. myserver:21 or myserver:ftp
func Dial(host string) (*Connection, os.Error) {
  if host == "" {
    return nil, os.NewError("FTP Connection Error: Host can not be blank!")
  }
  if !hasPort(host) {
    return nil, os.NewError("FTP Connection Error: Host must have a port! e.g. host:21")
  }
  conn, err := net.Dial("tcp", "", host)
  if err != nil {
    return nil, err
  }
  return &Connection{conn}, nil
}


// Reused from src/pkg/http/client.go
// Given a string of the form "host", "host:port", or "[ipv6::address]:port",
// return true if the string includes a port.
func hasPort(s string) bool { return strings.LastIndex(s, ":") > strings.LastIndex(s, "]") }

