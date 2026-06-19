package driver

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
    "github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	sdkModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
)

const (
	tableHoldingRegisters = "HOLDING_REGISTERS"
	tableInputRegisters   = "INPUT_REGISTERS"
	tableCoils            = "COILS"
	tableDiscreteInputs   = "DISCRETES_INPUT"
)

func readResource(ctx context.Context, conn *tcpConnection, req sdkModels.CommandRequest) (*sdkModels.CommandValue, error) {
	attrs, err := getRegisterAttributes(req.Attributes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", req.DeviceResourceName, err)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if err := conn.ensureConnected(ctx); err != nil {
		return nil, err
	}

	var raw []byte
	switch attrs.PrimaryTable {
	case tableHoldingRegisters:
		raw, err = conn.client.ReadHoldingRegisters(ctx, attrs.StartingAddress, attrs.Length)
	case tableInputRegisters:
		raw, err = conn.client.ReadInputRegisters(ctx, attrs.StartingAddress, attrs.Length)
	case tableCoils:
		raw, err = conn.client.ReadCoils(ctx, attrs.StartingAddress, attrs.Length)
	case tableDiscreteInputs:
		raw, err = conn.client.ReadDiscreteInputs(ctx, attrs.StartingAddress, attrs.Length)
	default:
		return nil, fmt.Errorf("%s: unsupported primaryTable %q", req.DeviceResourceName, attrs.PrimaryTable)
	}
	if err != nil {
		conn.close() // force a redial on the next request after any I/O error
		return nil, fmt.Errorf("%s: modbus read failed: %w", req.DeviceResourceName, err)
	}

	raw = swapBytes(raw, attrs.IsByteSwap, attrs.IsWordSwap)

	value, err := decodeRaw(raw, attrs.RawType)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", req.DeviceResourceName, err)
	}

	rawValueType, err := rawTypeToValueType(attrs.RawType)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", req.DeviceResourceName, err)
	}

	return sdkModels.NewCommandValue(req.DeviceResourceName, rawValueType, value)
}

func writeResource(ctx context.Context, conn *tcpConnection, req sdkModels.CommandRequest, param *sdkModels.CommandValue) error {
	attrs, err := getRegisterAttributes(req.Attributes)
	if err != nil {
		return fmt.Errorf("%s: %w", req.DeviceResourceName, err)
	}

	raw, err := encodeRaw(param, attrs.RawType)
	if err != nil {
		return fmt.Errorf("%s: %w", req.DeviceResourceName, err)
	}
	raw = swapBytes(raw, attrs.IsByteSwap, attrs.IsWordSwap)

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if err := conn.ensureConnected(ctx); err != nil {
		return err
	}

	switch attrs.PrimaryTable {
	case tableHoldingRegisters:
		_, err = conn.client.WriteMultipleRegisters(ctx, attrs.StartingAddress, attrs.Length, raw)
	case tableCoils:
		_, err = conn.client.WriteMultipleCoils(ctx, attrs.StartingAddress, attrs.Length, raw)
	default:
		return fmt.Errorf("%s: writes are not supported on table %q", req.DeviceResourceName, attrs.PrimaryTable)
	}
	if err != nil {
		conn.close()
		return fmt.Errorf("%s: modbus write failed: %w", req.DeviceResourceName, err)
	}
	return nil
}

func decodeRaw(raw []byte, rawType string) (interface{}, error) {
	switch rawType {
	case "BOOL":
		if len(raw) == 0 {
			return nil, fmt.Errorf("empty payload for BOOL")
		}
		return raw[0]&0x01 == 0x01, nil
	case "INT16":
		return int16(binary.BigEndian.Uint16(raw)), nil
	case "UINT16":
		return binary.BigEndian.Uint16(raw), nil
	case "INT32":
		return int32(binary.BigEndian.Uint32(raw)), nil
	case "UINT32":
		return binary.BigEndian.Uint32(raw), nil
	case "FLOAT32":
		return math.Float32frombits(binary.BigEndian.Uint32(raw)), nil
	case "FLOAT64":
		return math.Float64frombits(binary.BigEndian.Uint64(raw)), nil
	default:
		return nil, fmt.Errorf("unsupported rawType %q", rawType)
	}
}

func rawTypeToValueType(rawType string) (string, error) {
	switch rawType {
	case "BOOL":
		return common.ValueTypeBool, nil
	case "INT16":
		return common.ValueTypeInt16, nil
	case "UINT16":
		return common.ValueTypeUint16, nil
	case "INT32":
		return common.ValueTypeInt32, nil
	case "UINT32":
		return common.ValueTypeUint32, nil
	case "FLOAT32":
		return common.ValueTypeFloat32, nil
	case "FLOAT64":
		return common.ValueTypeFloat64, nil
	default:
		return "", fmt.Errorf("unsupported rawType %q", rawType)
	}
}

func encodeRaw(cv *sdkModels.CommandValue, rawType string) ([]byte, error) {
	switch rawType {
	case "BOOL":
		v, err := cv.BoolValue()
		if err != nil {
			return nil, err
		}
		if v {
			return []byte{1}, nil
		}
		return []byte{0}, nil
	case "INT16":
		v, err := cv.Int16Value()
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.AppendUint16(nil, uint16(v)), nil
	case "UINT16":
		v, err := cv.Uint16Value()
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.AppendUint16(nil, v), nil
	case "INT32":
		v, err := cv.Int32Value()
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.AppendUint32(nil, uint32(v)), nil
	case "UINT32":
		v, err := cv.Uint32Value()
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.AppendUint32(nil, v), nil
	case "FLOAT32":
		v, err := cv.Float32Value()
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.AppendUint32(nil, math.Float32bits(v)), nil
	case "FLOAT64":
		v, err := cv.Float64Value()
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.AppendUint64(nil, math.Float64bits(v)), nil
	default:
		return nil, fmt.Errorf("unsupported rawType %q", rawType)
	}
}

// swapBytes applies the profile's isByteSwap / isWordSwap attributes, since
// Modbus devices disagree on register/byte ordering for multi-register values.
func swapBytes(raw []byte, byteSwap, wordSwap bool) []byte {
	if !byteSwap && !wordSwap {
		return raw
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	if byteSwap {
		for i := 0; i+1 < len(out); i += 2 {
			out[i], out[i+1] = out[i+1], out[i]
		}
	}
	if wordSwap {
		for i := 0; i+3 < len(out); i += 4 {
			out[i], out[i+2] = out[i+2], out[i]
			out[i+1], out[i+3] = out[i+3], out[i+1]
		}
	}
	return out
}