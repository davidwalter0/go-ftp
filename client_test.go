package ftp

import (
	/*"fmt"*/
	"fmt"
	"github.com/davidwalter0/go-cfg"
	"io/ioutil"
	"testing"
)

// Allow unit tests to use a configurable server
// Example format
// source or set environment variables

// export FTP_FILENAME=Example.txt
// export FTP_HOST=192.168.0.12
// export FTP_PORT=2121
// export FTP_USER=ftp
// export FTP_PASSWORD=ftp

var _timeout = uint(5) // seconds
var _text = `
type Ftp struct {
	Host     string
	Port     string
	User     string
	Password string
	Filename string
}

var _ftpcfg = &Ftp{}
var _connectString string

func init() {
	var err error

	var sti = &cfg.StructInfo{
		StructPtr: _ftpcfg,
	}

	if err = sti.Parse(); err != nil {
		fmt.Printf("%v\n", err)
	}
	_connectString = fmt.Sprintf("%s:%s", _ftpcfg.Host, _ftpcfg.Port)
}
`

type Ftp struct {
	Host     string
	Port     string
	User     string
	Password string
	Filename string
}

var _ftpcfg = &Ftp{}
var _connectString string

func init() {
	var err error

	var sti = &cfg.StructInfo{
		StructPtr: _ftpcfg,
	}

	if err = sti.Parse(); err != nil {
		fmt.Printf("%v\n", err)
	}
	_connectString = fmt.Sprintf("%s:%s", _ftpcfg.Host, _ftpcfg.Port)
}

// TODO: Mock out network access.  Tests currently require an FTP server
// running on localhost.

func TestDial(t *testing.T) {
	_, err := Dial("")
	if err == nil {
		t.Error("Dial() should return an error on blank 'host'!")
	}
	_, err = Dial(_connectString)
	fmt.Println("TestDial")
	if err != nil {
		t.Error(err)
	}
}

func TestLogin(t *testing.T) {
	conn, err := Dial(_connectString)
	fmt.Println("TestLogin")
	defer conn.Logout()
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Login("anonymous", "anonymous@")
	if err != nil {
		t.Error(err)
	}
	err = conn.Login("", "anonymous@")
	if err == nil {
		t.Error("Login() should return an error on blank 'user'!")
	}
	err = conn.Login("anonymous", "")
	if err == nil {
		t.Error("Login() should return an error on blank 'password'!")
	}
}

func TestLogout(t *testing.T) {
	conn, err := Dial(_connectString)
	fmt.Println("TestLogout")
	defer conn.Logout()
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Login("anonymous", "anonymous@")
	if err != nil {
		t.Error(err)
	}
	err = conn.Logout()
	if err != nil {
		t.Error(err)
	}
}

func TestCmd(t *testing.T) {
	conn, err := Dial(_connectString)
	fmt.Println("TestCmd")
	defer conn.Logout()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = conn.Cmd("USER", "anonymous")
	if err != nil {
		t.Error(err)
	}
}

func TestExtractDataPort(t *testing.T) {
	test_string := "227 Entering Passive Mode (127,0,0,1,205,238)."
	port, err := extractDataPort(test_string)
	if err != nil {
		t.Error(err)
	}
	if port != 52718 {
		t.Error("Failed port calculation! Expected 52718, got", port)
	}
}

func TestCheckResponseCode(t *testing.T) {
	// Success
	err := checkResponseCode(2, 230)
	if err != nil {
		t.Error("Should return nil if response code matches! ExpectCode: 2  ActualCode: 230")
	}
	err = checkResponseCode(23, 230)
	if err != nil {
		t.Error("Should return nil if response code matches! ExpectCode: 23  ActualCode: 230")
	}
	err = checkResponseCode(230, 230)
	if err != nil {
		t.Error("Should return nil if response code matches! ExpectCode: 230  ActualCode: 230")
	}

	// Failure
	err = checkResponseCode(2, 500)
	if err == nil {
		t.Error("Should return error if response code doesn't match (ExpectCode: 2  ActualCode: 500)!")
	}
	err = checkResponseCode(23, 500)
	if err == nil {
		t.Error("Should return error if response code doesn't match (ExpectCode: 23  ActualCode: 500)!")
	}
	err = checkResponseCode(230, 500)
	if err == nil {
		t.Error("Should return error if response code doesn't match (ExpectCode: 230  ActualCode: 500)!")
	}
}

func TestUpload(t *testing.T) {
	conn, err := Dial(_connectString)
	fmt.Println("TestUpload")
	defer conn.Logout()
	if err != nil {
		t.Fatal(err)
	}
	// err = conn.Login("anonymous", "anonymous@")
	err = conn.Login(_ftpcfg.User, _ftpcfg.Password)
	if err != nil {
		t.Error(err)
	}
	if false {
		err = ioutil.WriteFile("upload_file.txt", []byte(_text), 0666)
		if err != nil {
			t.Error(err)
		}
	}
	// err = conn.UploadFile("upload_file.txt", "/up/uploaded.txt", BINARY)
	err = conn.UploadFile("client_test.go", "/up/uploaded.txt", BINARY, _timeout)
	if err != nil {
		t.Error(err)
	}
}

func TestDownload(t *testing.T) {
	conn, err := Dial(_connectString)
	fmt.Println("TestDownload")
	defer conn.Logout()
	if err != nil {
		t.Fatal(err)
	}
	// err = conn.Login("anonymous", "anonymous@")
	err = conn.Login(_ftpcfg.User, _ftpcfg.Password)
	if err != nil {
		t.Error(err)
	}

	err = conn.DownloadFile("/up/uploaded.txt", "./download_test.txt", BINARY, _timeout)
	if err != nil {
		t.Error(err)
	}
}
