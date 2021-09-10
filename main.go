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

var serialSettings = &monitor.PortDescriptor{
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
}

var openedPort serial.Port

func main() {
	args.Parse()
	if args.ShowVersion {
		fmt.Printf("%s\n", version.VersionInfo)
		return
	}

	serialMonitor := &SerialMonitor{}
	monitorServer := monitor.NewServer(serialMonitor)
	if err := monitorServer.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}

// SerialMonitor is the implementation of the serial ports pluggable-monitor
type SerialMonitor struct {
	closeChan chan<- bool //TODO maybe useless
}

// Hello is the handler for the pluggable-monitor HELLO command
func (d *SerialMonitor) Hello(userAgent string, protocol int) error {
	return nil
}

// Describe is the handler for the pluggable-monitor DESCRIBE command
func (d *SerialMonitor) Describe() (*monitor.PortDescriptor, error) {
	return serialSettings, nil
}

// Configure is the handler for the pluggable-monitor CONFIGURE command
func (d *SerialMonitor) Configure(parameterName string, value string) error {
	if serialSettings.ConfigurationParameter[parameterName] == nil {
		return fmt.Errorf("could not find parameter named %s", parameterName)
	}
	values := serialSettings.ConfigurationParameter[parameterName].Values
	for _, i := range values {
		if i == value {
			serialSettings.ConfigurationParameter[parameterName].Selected = value
			if openedPort != nil {
				err := openedPort.SetMode(getMode())
				if err != nil {
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
	if openedPort != nil {
		return nil, fmt.Errorf("port already opened: %s", boardPort)
	}
	openedPort, err := serial.Open(boardPort, getMode())
	if err != nil {
		fmt.Print(boardPort)
		openedPort = nil
		return nil, errors.New(err.Error())

	}
	return openedPort, nil
}

// Close is the handler for the pluggable-monitor CLOSE command
func (d *SerialMonitor) Close() error {
	if openedPort == nil {
		return errors.New("port already closed")
	}
	openedPort.Close()
	openedPort = nil
	return nil
}

// Quit is the handler for the pluggable-monitor QUIT command
func (d *SerialMonitor) Quit() {}

func getMode() *serial.Mode {
	baud, _ := strconv.Atoi(serialSettings.ConfigurationParameter["baudrate"].Selected)
	parity, _ := strconv.Atoi(serialSettings.ConfigurationParameter["parity"].Selected)
	dataBits, _ := strconv.Atoi(serialSettings.ConfigurationParameter["bits"].Selected)
	stopBits, _ := strconv.Atoi(serialSettings.ConfigurationParameter["stop_bits"].Selected)

	mode := &serial.Mode{
		BaudRate: baud,
		Parity:   serial.Parity(parity),
		DataBits: dataBits,
		StopBits: serial.StopBits(stopBits),
	}
	return mode
}
