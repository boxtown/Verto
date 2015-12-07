package verto

import (
	"errors"
	"net"
	"time"
)

// ErrStopped gets returned by stoppable listener when a stop
// signal is sent to the listener.
var ErrStopped = errors.New("listener stopped")

// StoppableListener is a TCPListener with the ability
// to do a clean stop.
type StoppableListener struct {
	*net.TCPListener
	stop chan int
}

// WrapListener wraps an existing listener as a new StoppableListener. Currently
// only supports net.TCPListener pointers for wrapping.
func WrapListener(listener net.Listener) (*StoppableListener, error) {
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return nil, errors.New("cannot wrap listener")
	}

	wrappedListener := StoppableListener{
		TCPListener: tcpListener,
		stop:        make(chan int)}

	return &wrappedListener, nil
}

// Accept wraps the accept function and polls for a stop command
// every second.
func (sl *StoppableListener) Accept() (net.Conn, error) {
	for {
		sl.SetDeadline(time.Now().Add(time.Second))
		netConn, err := sl.TCPListener.Accept()

		select {
		case <-sl.stop:
			return nil, ErrStopped
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

// Close sends a stop command to the listener.
func (sl *StoppableListener) Close() error {
	select {
	case <-sl.stop:
	default:
		close(sl.stop)
	}
	return nil
}
