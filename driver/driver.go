package driver

import (
	"context"
	"fmt"
	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	sdkModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
)

type Driver struct {
	lc    logger.LoggingClient
	sdk   interfaces.DeviceServiceSDK
	conns *connectionManager
}

// Compile-time check: catches any ProtocolDriver signature drift at build
// time instead of as a vague wiring error in main.go.
var _ interfaces.ProtocolDriver = (*Driver)(nil)

func NewDriver() *Driver {
	return &Driver{conns: newConnectionManager()}
}

func (d *Driver) Initialize(sdk interfaces.DeviceServiceSDK) error {
	d.sdk = sdk
	d.lc = sdk.LoggingClient()
	d.lc.Info("Modbus-TCP driver initialized")
	return nil
}

func (d *Driver) Start() error {
	return nil
}

func (d *Driver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []sdkModels.CommandRequest) ([]*sdkModels.CommandValue, error) {
	info, err := getTCPDeviceInfo(protocols)
	if err != nil {
		return nil, fmt.Errorf("HandleReadCommands %s: %w", deviceName, err)
	}

	conn := d.conns.get(deviceName, *info)
	ctx := context.Background()
	results := make([]*sdkModels.CommandValue, len(reqs))

	for i, req := range reqs {
		cv, err := readResource(ctx, conn, req)
		if err != nil {
			return nil, fmt.Errorf("HandleReadCommands %s: %w", deviceName, err)
		}
		results[i] = cv
	}

	return results, nil
}

func (d *Driver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []sdkModels.CommandRequest, params []*sdkModels.CommandValue) error {
	info, err := getTCPDeviceInfo(protocols)
	if err != nil {
		return fmt.Errorf("HandleWriteCommands %s: %w", deviceName, err)
	}

	conn := d.conns.get(deviceName, *info)
	ctx := context.Background()

	for i, req := range reqs {
		if err := writeResource(ctx, conn, req, params[i]); err != nil {
			return fmt.Errorf("HandleWriteCommands %s: %w", deviceName, err)
		}
	}

	return nil
}

func (d *Driver) Stop(force bool) error {
	d.conns.closeAll()
	return nil
}

func (d *Driver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	info, err := getTCPDeviceInfo(protocols)
	if err != nil {
		return fmt.Errorf("AddDevice %s: %w", deviceName, err)
	}

	d.conns.get(deviceName, *info)
	d.lc.Infof("AddDevice %s -> %s:%d (unit %d)", deviceName, info.Host, info.Port, info.UnitID)
	return nil
}

func (d *Driver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	info, err := getTCPDeviceInfo(protocols)
	if err != nil {
		return fmt.Errorf("UpdateDevice %s: %w", deviceName, err)
	}

	d.conns.remove(deviceName) // Host/Port/UnitID may have changed
	d.conns.get(deviceName, *info)
	return nil
}

func (d *Driver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.conns.remove(deviceName)
	return nil
}

func (d *Driver) Discover() error {
	return fmt.Errorf("discovery is not implemented for this service")
}

func (d *Driver) ValidateDevice(device models.Device) error {
	if _, err := getTCPDeviceInfo(device.Protocols); err != nil {
		return fmt.Errorf("device %s failed validation: %w", device.Name, err)
	}

	if _, ok := device.Protocols["Modbus-RTU"]; ok {
		return fmt.Errorf("device %s has a Modbus-RTU block; this service is TCP-only", device.Name)
	}

	return nil
}
