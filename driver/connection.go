package driver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/grid-x/modbus"
)

// tcpConnection wraps one TCP socket to one physical device. Modbus has no
// request pipelining, so every read/write against this connection must hold
// mu for the full round trip.
type tcpConnection struct {
	mu      sync.Mutex
	handler *modbus.TCPClientHandler
	client  modbus.Client
	info    TCPDeviceInfo
}

func newTCPConnection(info TCPDeviceInfo) *tcpConnection {
	return &tcpConnection{info: info}
}

// ensureConnected lazily dials the device. Call only while holding mu.
func (c *tcpConnection) ensureConnected(ctx context.Context) error {
	if c.handler != nil {
		return nil
	}

	handler := modbus.NewTCPClientHandler(fmt.Sprintf("%s:%d", c.info.Host, c.info.Port))
	handler.Timeout = 5 * time.Second
	handler.SlaveID = c.info.UnitID

	if err := handler.Connect(ctx); err != nil {
		return fmt.Errorf("connect to %s:%d failed: %w", c.info.Host, c.info.Port, err)
	}

	c.handler = handler
	c.client = modbus.NewClient(handler)
	return nil
}

// close drops the socket. The next ensureConnected call will redial.
// Call only while holding mu.
func (c *tcpConnection) close() {
	if c.handler != nil {
		c.handler.Close()
		c.handler = nil
		c.client = nil
	}
}

// connectionManager is one tcpConnection per device name, so devices never
// block on each other's I/O.
type connectionManager struct {
	mu    sync.Mutex
	conns map[string]*tcpConnection
}

func newConnectionManager() *connectionManager {
	return &connectionManager{conns: make(map[string]*tcpConnection)}
}

func (m *connectionManager) get(deviceName string, info TCPDeviceInfo) *tcpConnection {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, ok := m.conns[deviceName]; ok {
		return conn
	}
	conn := newTCPConnection(info)
	m.conns[deviceName] = conn
	return conn
}

func (m *connectionManager) remove(deviceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, ok := m.conns[deviceName]; ok {
		conn.mu.Lock()
		conn.close()
		conn.mu.Unlock()
		delete(m.conns, deviceName)
	}
}

func (m *connectionManager) closeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, conn := range m.conns {
		conn.mu.Lock()
		conn.close()
		conn.mu.Unlock()
		delete(m.conns, name)
	}
}