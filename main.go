//
// This file is part of serial-monitor.
//
// Copyright 2018-2021 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to modify or
// otherwise use the software for commercial activities involving the Arduino
// software without disclosing the source code of your own applications. To purchase
// a commercial license, send an email to license@arduino.cc.
//

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	monitor "github.com/arduino/pluggable-monitor-protocol-handler"
	"github.com/arduino/serial-monitor/args"
	"github.com/arduino/serial-monitor/version"
	"go.bug.st/serial"
)

func main() {
	args.Parse()
	if args.ShowVersion {
		fmt.Printf("%s\n", version.VersionInfo)
		return
	}

	monitorServer := monitor.NewServer(NewSerialMonitor())
	if err := monitorServer.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}

// SerialMonitor is the implementation of the serial ports pluggable-monitor
type SerialMonitor struct {
	serialPort     serial.Port
	serialSettings *monitor.PortDescriptor
	openedPort     bool
}

// NewSerialMonitor will initialize and return a SerialMonitor
func NewSerialMonitor() *SerialMonitor {
	return &SerialMonitor{
		serialSettings: &monitor.PortDescriptor{
			Protocol: "serial",
			ConfigurationParameter: map[string]*monitor.PortParameterDescriptor{
				"baudrate": {
					Label:    "Baudrate",
					Type:     "enum",
					Values:   []string{"300", "600", "750", "1200", "2400", "4800", "9600", "19200", "38400", "57600", "115200", "230400", "460800", "500000", "921600", "1000000", "2000000"},
					Selected: "9600",
				},
				"parity": {
					Label:    "Parity",
					Type:     "enum",
					Values:   []string{"N", "E", "O", "M", "S"},
					Selected: "N",
				},
				"bits": {
					Label:    "Data bits",
					Type:     "enum",
					Values:   []string{"5", "6", "7", "8", "9"},
					Selected: "8",
				},
				"stop_bits": {
					Label:    "Stop bits",
					Type:     "enum",
					Values:   []string{"1", "1.5", "2"},
					Selected: "1",
				},
			},
		},
		openedPort: false,
	}
}

// Hello is the handler for the pluggable-monitor HELLO command
func (d *SerialMonitor) Hello(userAgent string, protocol int) error {
	return nil
}

// Describe is the handler for the pluggable-monitor DESCRIBE command
func (d *SerialMonitor) Describe() (*monitor.PortDescriptor, error) {
	return d.serialSettings, nil
}

// Configure is the handler for the pluggable-monitor CONFIGURE command
func (d *SerialMonitor) Configure(parameterName string, value string) error {
	if d.serialSettings.ConfigurationParameter[parameterName] == nil {
		return fmt.Errorf("could not find parameter named %s", parameterName)
	}
	values := d.serialSettings.ConfigurationParameter[parameterName].Values
	for _, i := range values {
		if i == value {
			oldValue := d.serialSettings.ConfigurationParameter[parameterName].Selected
			d.serialSettings.ConfigurationParameter[parameterName].Selected = value
			if d.openedPort {
				err := d.serialPort.SetMode(d.getMode())
				if err != nil {
					d.serialSettings.ConfigurationParameter[parameterName].Selected = oldValue
					return errors.New(err.Error())
				}
			}
			return nil
		}
	}
	return fmt.Errorf("invalid value for parameter %s: %s", parameterName, value)
}

// Open is the handler for the pluggable-monitor OPEN command
func (d *SerialMonitor) Open(boardPort string) (io.ReadWriter, error) {
	if d.openedPort {
		return nil, fmt.Errorf("port already opened: %s", boardPort)
	}
	serialPort, err := serial.Open(boardPort, d.getMode())
	if err != nil {
		return nil, err

	}
	d.openedPort = true
	d.serialPort = serialPort
	return d.serialPort, nil
}

// Close is the handler for the pluggable-monitor CLOSE command
func (d *SerialMonitor) Close() error {
	if !d.openedPort {
		return errors.New("port already closed")
	}
	d.serialPort.Close()
	d.openedPort = false
	return nil
}

// Quit is the handler for the pluggable-monitor QUIT command
func (d *SerialMonitor) Quit() {}

func (d *SerialMonitor) getMode() *serial.Mode {
	baud, _ := strconv.Atoi(d.serialSettings.ConfigurationParameter["baudrate"].Selected)
	var parity serial.Parity
	switch d.serialSettings.ConfigurationParameter["parity"].Selected {
	case "N":
		parity = serial.NoParity
	case "E":
		parity = serial.EvenParity
	case "O":
		parity = serial.OddParity
	case "M":
		parity = serial.MarkParity
	case "S":
		parity = serial.SpaceParity
	}
	dataBits, _ := strconv.Atoi(d.serialSettings.ConfigurationParameter["bits"].Selected)
	var stopBits serial.StopBits
	switch d.serialSettings.ConfigurationParameter["stop_bits"].Selected {
	case "1":
		stopBits = serial.OneStopBit
	case "1.5":
		stopBits = serial.OnePointFiveStopBits
	case "2":
		stopBits = serial.TwoStopBits
	}

	mode := &serial.Mode{
		BaudRate: baud,
		Parity:   parity,
		DataBits: dataBits,
		StopBits: stopBits,
	}
	return mode
}
