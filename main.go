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
	"golang.org/x/exp/slices"
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
					Label: "Baudrate",
					Type:  "enum",
					Values: []string{
						"300", "600", "750",
						"1200", "2400", "4800", "9600",
						"19200", "31250", "38400", "57600", "74880",
						"115200", "230400", "250000", "460800", "500000", "921600",
						"1000000", "2000000"},
					Selected: "9600",
				},
				"parity": {
					Label:    "Parity",
					Type:     "enum",
					Values:   []string{"none", "even", "odd", "mark", "space"},
					Selected: "none",
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
				"rts": {
					Label:    "RTS",
					Type:     "enum",
					Values:   []string{"on", "off"},
					Selected: "on",
				},
				"dtr": {
					Label:    "DTR",
					Type:     "enum",
					Values:   []string{"on", "off"},
					Selected: "on",
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
	parameter, ok := d.serialSettings.ConfigurationParameter[parameterName]
	if !ok {
		return fmt.Errorf("could not find parameter named %s", parameterName)
	}
	if !slices.Contains(parameter.Values, value) {
		return fmt.Errorf("invalid value for parameter %s: %s", parameterName, value)
	}
	// Set configuration
	oldValue := parameter.Selected
	parameter.Selected = value

	// Apply configuration to port
	var configErr error
	if d.openedPort {
		switch parameterName {
		case "baudrate", "parity", "bits", "stop_bits":
			configErr = d.serialPort.SetMode(d.getMode())
		case "dtr":
			configErr = d.serialPort.SetDTR(d.getDTR())
		case "rts":
			configErr = d.serialPort.SetRTS(d.getRTS())
		default:
			// Should never happen
			panic("Invalid parameter: " + parameterName)
		}
	}

	// If configuration failed, rollback settings
	if configErr != nil {
		parameter.Selected = oldValue
		return configErr
	}
	return nil
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

	// Clean up residual data in IO buffers
	_ = serialPort.ResetInputBuffer() // do not error if resetting buffers fails
	_ = serialPort.ResetOutputBuffer()

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
	case "None":
		parity = serial.NoParity
	case "Even":
		parity = serial.EvenParity
	case "Odd":
		parity = serial.OddParity
	case "Mark":
		parity = serial.MarkParity
	case "Space":
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
		InitialStatusBits: &serial.ModemOutputBits{
			DTR: d.getDTR(),
			RTS: d.getRTS(),
		},
	}
	return mode
}

func (d *SerialMonitor) getDTR() bool {
	return d.serialSettings.ConfigurationParameter["dtr"].Selected == "on"
}

func (d *SerialMonitor) getRTS() bool {
	return d.serialSettings.ConfigurationParameter["rts"].Selected == "on"
}
