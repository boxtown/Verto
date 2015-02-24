package verto

import (
	"errors"
	"net"
	"time"
)

var StoppedError = errors.New("Listener stopped.")

type StoppableListener struct {
	*net.TCPListener
	stop chan int
}

// Wraps an existing listener as a new StoppableListener. Currently
// only supports net.TCPListener pointers for wrapping.
func WrapListener(listener net.Listener) (*StoppableListener, error) {
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return nil, errors.New("Cannot wrap listener.")
	}

	wrappedListener := StoppableListener{
		TCPListener: tcpListener,
		stop:        make(chan int)}

	return &wrappedListener, nil
}

// Wrapped accept function that polls for a stop command
// every second.
func (sl *StoppableListener) Accept() (net.Conn, error) {
	for {
		sl.SetDeadline(time.Now().Add(time.Second))

		netConn, err := sl.TCPListener.Accept()

		select {
		case <-sl.stop:
			return nil, StoppedError
		default:
		}

		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() && netErr.Temporary() {
				continue
			}
		}

		return netConn, err
	}
}

// Sends a stop command to the listener.
func (sl *StoppableListener) Stop() {
	close(sl.stop)
}
