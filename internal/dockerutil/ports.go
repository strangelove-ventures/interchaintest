package dockerutil

import (
	"fmt"
	"net"
	"sync"

	"github.com/docker/go-connections/nat"
)

var mu sync.RWMutex

type Listeners []net.Listener

func (l Listeners) CloseAll() {
	for _, listener := range l {
		listener.Close()
	}
}

// openListenerOnFreePort opens the next free port
func openListenerOnFreePort() (*net.TCPListener, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	mu.Lock()
	defer mu.Unlock()
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}

	return l, nil
}

// nextAvailablePort generates a docker PortBinding by finding the next available port.
// The listener will be closed in the case of an error, otherwise it will be left open.
// This allows multiple nextAvailablePort calls to find multiple available ports
// before closing them so they are available for the PortBinding.
func nextAvailablePort() (nat.PortBinding, *net.TCPListener, error) {
	l, err := openListenerOnFreePort()
	if err != nil {
		l.Close()
		return nat.PortBinding{}, nil, err
	}

	return nat.PortBinding{
		HostIP:   "0.0.0.0",
		HostPort: fmt.Sprint(l.Addr().(*net.TCPAddr).Port),
	}, l, nil
}

// GeneratePortBindings will find open ports on the local
// machine and create a PortBinding for every port in the portSet.
func GeneratePortBindings(portSet nat.PortSet) (nat.PortMap, Listeners, error) {
	m := make(nat.PortMap)
	listeners := make(Listeners, 0, len(portSet))

	for p := range portSet {
		pb, l, err := nextAvailablePort()
		if err != nil {
			listeners.CloseAll()
			return nat.PortMap{}, nil, err
		}
		listeners = append(listeners, l)
		m[p] = []nat.PortBinding{pb}
	}

	return m, listeners, nil
}
