package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	TextMessage   = 1
	BinaryMessage = 2
	CloseMessage  = 8
	PingMessage   = 9
	PongMessage   = 10
)

var errClosed = errors.New("websocket connection closed")

type rawConn struct {
	conn        net.Conn
	reader      *bufio.Reader
	writeMu     sync.Mutex
	readLimit   int64
	pongHandler func(string) error
	closed      bool
	closeMu     sync.Mutex
}

func upgrade(writer http.ResponseWriter, request *http.Request) (*rawConn, error) {
	if !strings.EqualFold(request.Header.Get("Upgrade"), "websocket") ||
		!headerContains(request.Header.Get("Connection"), "upgrade") {
		return nil, errors.New("not a websocket upgrade request")
	}
	key := strings.TrimSpace(request.Header.Get("Sec-WebSocket-Key"))
	if key == "" || request.Header.Get("Sec-WebSocket-Version") != "13" {
		return nil, errors.New("invalid websocket handshake")
	}
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		return nil, errors.New("http server does not support hijacking")
	}
	connection, readWriter, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}
	accept := websocketAccept(key)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := readWriter.WriteString(response); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if err := readWriter.Flush(); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return &rawConn{conn: connection, reader: readWriter.Reader, readLimit: 32 * 1024}, nil
}

func headerContains(value, token string) bool {
	for _, item := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(item), token) {
			return true
		}
	}
	return false
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func (c *rawConn) SetReadLimit(limit int64)                 { c.readLimit = limit }
func (c *rawConn) SetReadDeadline(deadline time.Time) error { return c.conn.SetReadDeadline(deadline) }
func (c *rawConn) SetWriteDeadline(deadline time.Time) error {
	return c.conn.SetWriteDeadline(deadline)
}
func (c *rawConn) SetPongHandler(handler func(string) error) { c.pongHandler = handler }

func (c *rawConn) ReadMessage() (int, []byte, error) {
	var assembled []byte
	messageType := 0
	for {
		fin, opcode, payload, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}
		switch opcode {
		case CloseMessage:
			_ = c.WriteMessage(CloseMessage, nil)
			return 0, nil, errClosed
		case PingMessage:
			if err := c.WriteMessage(PongMessage, payload); err != nil {
				return 0, nil, err
			}
			continue
		case PongMessage:
			if c.pongHandler != nil {
				if err := c.pongHandler(string(payload)); err != nil {
					return 0, nil, err
				}
			}
			continue
		case TextMessage, BinaryMessage:
			if messageType != 0 {
				return 0, nil, errors.New("unexpected data frame")
			}
			messageType = opcode
			assembled = append(assembled, payload...)
		case 0:
			if messageType == 0 {
				return 0, nil, errors.New("unexpected continuation frame")
			}
			assembled = append(assembled, payload...)
		default:
			return 0, nil, fmt.Errorf("unsupported websocket opcode %d", opcode)
		}
		if c.readLimit > 0 && int64(len(assembled)) > c.readLimit {
			return 0, nil, errors.New("websocket message too large")
		}
		if fin && messageType != 0 {
			return messageType, assembled, nil
		}
	}
}

func (c *rawConn) readFrame() (bool, int, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return false, 0, nil, err
	}
	fin := header[0]&0x80 != 0
	if header[0]&0x70 != 0 {
		return false, 0, nil, errors.New("websocket extensions are not supported")
	}
	opcode := int(header[0] & 0x0f)
	masked := header[1]&0x80 != 0
	if !masked {
		return false, 0, nil, errors.New("client websocket frames must be masked")
	}
	length := uint64(header[1] & 0x7f)
	switch length {
	case 126:
		extra := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, extra); err != nil {
			return false, 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(extra))
	case 127:
		extra := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, extra); err != nil {
			return false, 0, nil, err
		}
		length = binary.BigEndian.Uint64(extra)
		if length&(1<<63) != 0 {
			return false, 0, nil, errors.New("invalid websocket payload length")
		}
	}
	if c.readLimit > 0 && int64(length) > c.readLimit {
		return false, 0, nil, errors.New("websocket frame too large")
	}
	mask := make([]byte, 4)
	if _, err := io.ReadFull(c.reader, mask); err != nil {
		return false, 0, nil, err
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return false, 0, nil, err
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return fin, opcode, payload, nil
}

func (c *rawConn) WriteMessage(messageType int, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.closeMu.Lock()
	closed := c.closed
	c.closeMu.Unlock()
	if closed {
		return errClosed
	}
	if messageType < 0 || messageType > 15 {
		return errors.New("invalid websocket message type")
	}
	header := []byte{0x80 | byte(messageType)}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length <= 65535:
		header = append(header, 126, byte(length>>8), byte(length))
	default:
		header = append(header, 127)
		size := make([]byte, 8)
		binary.BigEndian.PutUint64(size, uint64(length))
		header = append(header, size...)
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := c.conn.Write(payload)
		return err
	}
	return nil
}

func (c *rawConn) Close() error {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return nil
	}
	c.closed = true
	c.closeMu.Unlock()
	return c.conn.Close()
}
