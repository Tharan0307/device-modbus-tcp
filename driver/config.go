package driver

import (
	"fmt"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/spf13/cast"
)

const protocolName = "modbus-tcp"

// TCPDeviceInfo holds the parsed Host/Port/UnitID for one device, pulled out
// of the generic protocols map the SDK hands to every driver callback.
type TCPDeviceInfo struct {
	Host   string
	Port   int	
	UnitID byte
}

func getTCPDeviceInfo(protocols map[string]models.ProtocolProperties) (*TCPDeviceInfo, error) {
	props, ok := protocols[protocolName]
	if !ok {
		return nil, fmt.Errorf("protocol properties missing %q block", protocolName)
	}

	host := cast.ToString(props["Host"])
	if host == "" {
		host = cast.ToString(props["Address"])
	}
	if host == "" {
		return nil, fmt.Errorf("%s.Host is empty or missing", protocolName)
	}

	port, err := cast.ToIntE(props["Port"])
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("%s.Port %v is invalid", protocolName, props["Port"])
	}

	unitID, err := cast.ToUint8E(props["UnitID"])
	if err != nil {
		return nil, fmt.Errorf("%s.UnitID %v is invalid", protocolName, props["UnitID"])
	}

	return &TCPDeviceInfo{Host: host, Port: port, UnitID: unitID}, nil
}

// RegisterAttributes is the protocol-agnostic per-resource info that lives in
// a device profile's deviceResources[].attributes block.
type RegisterAttributes struct {
	PrimaryTable    string // HOLDING_REGISTERS | INPUT_REGISTERS | COILS | DISCRETES_INPUT
	StartingAddress uint16
	Length          uint16
	RawType         string // INT16 | UINT16 | INT32 | UINT32 | FLOAT32 | FLOAT64 | BOOL
	IsByteSwap      bool
	IsWordSwap      bool
}

func getRegisterAttributes(attrs map[string]interface{}) (*RegisterAttributes, error) {
	table := cast.ToString(attrs["primaryTable"])
	if table == "" {
		return nil, fmt.Errorf("attribute primaryTable is required")
	}

	addr, err := cast.ToUint16E(attrs["startingAddress"])
	if err != nil {
		return nil, fmt.Errorf("attribute startingAddress is invalid: %w", err)
	}

	length, err := cast.ToUint16E(attrs["length"])
	if err != nil || length == 0 {
		return nil, fmt.Errorf("attribute length is invalid or zero")
	}

	return &RegisterAttributes{
		PrimaryTable:    table,
		StartingAddress: addr,
		Length:          length,
		RawType:         cast.ToString(attrs["rawType"]),
		IsByteSwap:      cast.ToBool(attrs["isByteSwap"]),
		IsWordSwap:      cast.ToBool(attrs["isWordSwap"]),
	}, nil
}