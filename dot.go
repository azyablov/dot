package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"time"
)

// Defaults
var lisPort string = "8853"
var tlsServer string = "8.8.8.8"

type DNSMsgLength uint16

type MessageID uint16

type FieldPopulationError struct {
	tOfErr int
	Err    error
}

type DNSMessage struct {
	Length DNSMsgLength
	ID     MessageID
	Body   []byte
}

func (e *FieldPopulationError) Error() string {
	var rStr string
	switch e.tOfErr {
	default:
		rStr = "Can't extract necessary field from the buffer."
		return rStr
	}
}

func (e *FieldPopulationError) Unwrap() error {
	return e.Err
}

func clientHandler(conn net.Conn) {
	// Defer closure
	defer conn.Close()
	// Setting readtimeout
	timeoutDurationRead := 2 * time.Second
	conn.SetReadDeadline(time.Now().Add(timeoutDurationRead))
	// Who is connected right now
	remoteAddr := conn.RemoteAddr().String()
	fmt.Println("Client connected from " + remoteAddr)

	// Craeting reader buffer
	cr := bufio.NewReader(conn)

	for {

		var recvCltMsg DNSMessage

		var data interface{}
		data = &recvCltMsg.Length
		err := readByteField(cr, 2, data)
		if err != nil {
			log.Println("Gacefully closing client connection\n", err)
			return
		}
		data = &recvCltMsg.ID
		err = readByteField(cr, 2, data)
		if err != nil {
			log.Println("Gacefully closing client connection\n", err)
			return
		}
		recvCltMsg.Body = make([]byte, recvCltMsg.Length-2)
		_, err = io.ReadFull(cr, recvCltMsg.Body)
		if err != nil {
			log.Println("Gacefully closing client connection\n", err)
			return
		}

		// rootCAPEM := `<root CA cert in PEM>`

		// rootCAs := x509.NewCertPool()
		// ok := rootCAs.AppendCertsFromPEM([]byte(rootCAPEM))
		// if !ok {
		// 	panic("failed to parse root certificate")
		// }
		tlsConf := &tls.Config{
			//	RootCAs: rootCAs, // TODO
			InsecureSkipVerify: true,
		}

		// Initiate TLS connection

		tlsConn, err := tls.Dial("tcp", tlsServer+":853", tlsConf)
		tlsConn.SetReadDeadline(time.Now().Add(timeoutDurationRead))
		tlsConn.SetWriteDeadline(time.Now().Add(timeoutDurationRead))
		if err != nil {
			log.Println(err)
			tlsConn.Close()
			return
		}
		// Defer closure
		defer tlsConn.Close()

		// Prepare message to send toward TLS server
		sendTLSMsg := []byte{}
		tmpbuff := bytes.NewBuffer(sendTLSMsg)
		err = binary.Write(tmpbuff, binary.BigEndian, recvCltMsg.Length)
		if err != nil {
			log.Println(err)
			return
		}
		binary.Write(tmpbuff, binary.BigEndian, recvCltMsg.ID)
		binary.Write(tmpbuff, binary.BigEndian, recvCltMsg.Body)

		// Sending message
		n, err := tlsConn.Write(tmpbuff.Bytes())
		if err != nil {
			log.Println(n, err)
			return
		}

		// Receiving response
		var recvTLSMsg DNSMessage
		data = &recvTLSMsg.Length
		err = readByteTLS(tlsConn, 2, data)
		if err != nil {
			log.Fatalln(err)
		}
		// ID
		data = &recvTLSMsg.ID
		err = readByteTLS(tlsConn, 2, data)
		if err != nil {
			log.Fatalln(err)
		}
		// Body
		recvTLSMsg.Body = make([]byte, recvTLSMsg.Length-2)
		_, err = io.ReadFull(tlsConn, recvTLSMsg.Body)
		if err != nil {
			log.Fatalln(err)
		}

		// Prepare message to send toward TLS server
		sendCltMsg := []byte{}
		tmpbuff = bytes.NewBuffer(sendCltMsg)
		err = binary.Write(tmpbuff, binary.BigEndian, recvTLSMsg.Length)
		if err != nil {
			log.Println(err)
			return
		}
		binary.Write(tmpbuff, binary.BigEndian, recvTLSMsg.ID)
		binary.Write(tmpbuff, binary.BigEndian, recvTLSMsg.Body)

		// Debug
		println("Client message length", recvCltMsg.Length)
		println("Client message ID", recvCltMsg.ID)
		fmt.Printf("Body: %v", recvCltMsg.Body)

		fmt.Printf("Received message length: %v\n", recvTLSMsg.Length)
		fmt.Printf("Received message ID: %v\n", recvTLSMsg.ID)
		fmt.Printf("Received message body: %v\n", recvTLSMsg.Body)

		// Sending clear text message toward client
		n, err = conn.Write(tmpbuff.Bytes())
		if err != nil {
			log.Println(n, err)
			return
		}
	}
}

func readByteField(reader io.Reader, nBytes uint, field interface{}) error {
	// Buffer to extact message length
	buf := make([]byte, nBytes)
	var e FieldPopulationError
	// reading predefined number of bytes
	for {
		_, err := io.ReadFull(reader, buf)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}
	fmt.Println(buf)
	// Creating teamporary buffer
	tmpbuff := bytes.NewBuffer(buf)
	// Populating field from buffer
	err := binary.Read(tmpbuff, binary.BigEndian, field)
	if err != nil {
		e.Err = err
		return &e
	}
	return nil
}

func readByteTLS(tlsConn *tls.Conn, nBytes uint16, field interface{}) error {
	var readBuf []byte = make([]byte, nBytes)
	var e FieldPopulationError
	for {
		n, err := tlsConn.Read(readBuf)
		if err != nil {
			//time.Sleep(time.Second)
			log.Println(n, err)
			continue
		}
		if n == 0 {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	// Creating teamporary buffer
	tmpbuff := bytes.NewBuffer(readBuf)
	// Populating field from buffer
	err := binary.Read(tmpbuff, binary.BigEndian, field)
	if err != nil {
		e.Err = err
		return &e
	}
	return nil
}

func main() {

	argLength := len(os.Args[1:])
	switch argLength {
	case 1:
		tlsServer = os.Args[1]
	case 2:
		tlsServer = os.Args[1]
		lisPort = os.Args[2]
	}

	fmt.Println("Launching server...")

	// Getting TCP listener
	ln, err := net.Listen("tcp", ":"+lisPort)
	if err != nil {
		bind_err, _ := regexp.MatchString("permission denied", err.Error())
		if bind_err {
			log.Fatalln("Not enougth privelege to listen on port: ", lisPort)
		}
		log.Fatalf("Can't start lisener")
	}
	defer ln.Close()

	// Handling incoming connections
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalln(err)
		}
		go clientHandler(conn)
	}
}
