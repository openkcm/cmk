package testutils

import (
	"errors"
	"net"
	"strconv"
)

var ErrFailedConversion = errors.New("failed to convert to TCPAddr")

// GetFreePort asks the kernel for a free open port that is ready to use.
func GetFreePort() (int, error) {
	var a *net.TCPAddr

	a, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err == nil {
		var l *net.TCPListener

		l, err = net.ListenTCP("tcp", a)
		if err == nil {
			defer l.Close()

			TCPAddr, ok := l.Addr().(*net.TCPAddr)

			if ok {
				return TCPAddr.Port, nil
			}

			err = ErrFailedConversion
		}
	}

	return 0, err
}

func GetFreePortString() (string, error) {
	portint, err := GetFreePort()
	if err == nil {
		return strconv.Itoa(portint), nil
	}

	return "0", err
}
