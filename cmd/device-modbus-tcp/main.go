// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018-2019 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// This package provides a simple example of a device service.
package main

import (
	"github.com/edgexfoundry/device-modbus-tcp"
	"github.com/edgexfoundry/device-modbus-tcp/driver"
	"github.com/edgexfoundry/device-sdk-go/v4/pkg/startup"
)

const (
	serviceName string = "device-modbus-tcp"
)

func main() {
    sd := driver.NewDriver()
    startup.Bootstrap(serviceName, device.Version, sd)
}