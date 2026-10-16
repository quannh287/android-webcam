package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const webcamPackage = "com.android.DeviceAsWebcam"

func run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, s)
	}
	return s, nil
}

type adbDevice struct{ Serial, State string }

func parseAdbDevices(out string) []adbDevice {
	var devs []adbDevice
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "*") {
			continue
		}
		devs = append(devs, adbDevice{Serial: f[0], State: f[1]})
	}
	return devs
}

func listDevices() ([]adbDevice, error) {
	out, err := run("adb", "devices")
	if err != nil {
		return nil, err
	}
	return parseAdbDevices(out), nil
}

func shell(serial string, cmd ...string) (string, error) {
	return run("adb", append([]string{"-s", serial, "shell"}, cmd...)...)
}

func getprop(serial, key string) string {
	out, _ := shell(serial, "getprop", key)
	return out
}

// usbFunctions excludes adb; "none" means charging only.
func usbFunctions(serial string) (string, error) {
	return shell(serial, "svc", "usb", "getFunctions")
}

func setUsbFunctions(serial, fn string) error {
	args := []string{"svc", "usb", "setFunctions"}
	if fn != "" && fn != "none" {
		args = append(args, fn)
	}
	// adb drops mid-command while the gadget re-enumerates, so the exit code is meaningless; verify instead.
	shell(serial, args...)
	want := fn
	if want == "" {
		want = "none"
	}
	var got string
	var err error
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(time.Second) {
		run("adb", "-s", serial, "wait-for-device")
		if got, err = usbFunctions(serial); err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("device did not come back after switching USB to %q: %w", want, err)
	}
	if got != want {
		return fmt.Errorf("device refused USB function %q (current: %q)", want, got)
	}
	return nil
}

const stayOnSetting = "stay_on_while_plugged_in"

// stayOnAny keeps the screen on for AC|USB|wireless; the Mac's USB-C port reports as AC, so "usb" alone isn't enough.
const stayOnAny = "7"

func stayOn(serial string) string {
	out, _ := shell(serial, "settings", "get", "global", stayOnSetting)
	return out
}

func setStayOn(serial, v string) error {
	_, err := shell(serial, "settings", "put", "global", stayOnSetting, v)
	return err
}

type battery struct {
	level    int
	tempC    float64
	charging bool
}

// parseBattery reads `dumpsys battery`; temperature is reported in tenths of °C.
func parseBattery(dump string) battery {
	var b battery
	for _, line := range strings.Split(dump, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if !ok {
			continue
		}
		switch {
		case k == "level":
			fmt.Sscan(v, &b.level)
		case k == "temperature":
			var t int
			fmt.Sscan(v, &t)
			b.tempC = float64(t) / 10
		case strings.HasSuffix(k, " powered") && v == "true":
			b.charging = true
		}
	}
	return b
}

func batteryInfo(serial string) battery {
	out, _ := shell(serial, "dumpsys", "battery")
	return parseBattery(out)
}

// ro.usb.uvc.enabled is empty on Pixel even when UVC works, so check the webcam service instead.
func supportsUvc(serial, current string) bool {
	if current == "uvc" {
		return true
	}
	out, _ := shell(serial, "pm", "list", "packages", webcamPackage)
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "package:"+webcamPackage {
			return true
		}
	}
	return false
}
