package ipc

import (
	"encoding/json"
	"errors"
	"net"
	"time"
)

// ErrNoDaemon means nothing is listening; the daemon is not running.
var ErrNoDaemon = errors.New("adzan daemon is not running")

// Handler turns a request into a response.
type Handler func(Request) Response

// Server accepts CLI connections for the lifetime of the daemon.
type Server struct {
	ln net.Listener
}

// Listen opens the local socket. It fails if a daemon is already running.
func Listen() (*Server, error) {
	l, err := listen()
	if err != nil {
		return nil, err
	}
	return &Server{ln: l}, nil
}

// Serve handles connections until Close is called.
func (s *Server) Serve(h Handler) {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // listener closed
		}
		go handle(conn, h)
	}
}

// Close stops listening and removes the socket/port file.
func (s *Server) Close() error {
	err := s.ln.Close()
	cleanup()
	return err
}

func handle(conn net.Conn, h Handler) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	resp := h(req)
	_ = json.NewEncoder(conn).Encode(resp)
}

// Send delivers one command to the daemon and returns its reply.
func Send(cmd string) (Response, error) {
	conn, err := dial()
	if err != nil {
		return Response{}, ErrNoDaemon
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	if err := json.NewEncoder(conn).Encode(Request{Command: cmd}); err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}

// Running reports whether a daemon is currently reachable.
func Running() bool {
	conn, err := dial()
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
