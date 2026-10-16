package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const usage = `Usage: android-webcam <command> [--device <serial>]

Commands:
  devices        List connected Android devices
  status         Show webcam status of a device
  doctor         Diagnose Android / USB / host layers
  enable         Switch the device to USB webcam (UVC) mode
  disable        Restore the USB configuration saved by enable
  capabilities   List resolutions/FPS exposed to the host
  test           Open a preview window (requires ffplay)
  camera [front|back]  Show or switch the streaming camera (phone must be unlocked)
  zoom [ratio]   Show zoom presets or pick the closest one, e.g. zoom 0.5
  hq             Toggle the phone's High quality mode
  watch          Enable webcam automatically whenever a device is plugged in`

// hotC is where sustained webcam use starts to risk thermal throttling.
const hotC = 40.0

const (
	msgNoDevice = `No Android device detected.

Please:
1. Connect the device using USB.
2. Enable USB debugging.
3. Accept the debugging authorization prompt.`
	msgUnsupported = `Android Webcam is not supported on this device.

Possible reasons:
- Android version does not support USB Webcam (needs Android 14+)
- Device manufacturer disabled UVC
- Device firmware does not expose the UVC function`
	msgHostMissing = `UVC mode was enabled, but the host OS did not detect
the Android camera.

Please check:
- USB cable
- USB connection mode
- Camera permissions
- Host camera services`
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	serial := fs.String("device", "", "device serial (required when several are connected)")
	fs.Parse(os.Args[2:])

	cmds := map[string]func() error{
		"devices":      cmdDevices,
		"status":       func() error { return cmdStatus(cmd, *serial) },
		"doctor":       func() error { return cmdDoctor(cmd, *serial) },
		"enable":       func() error { return cmdEnable(cmd, *serial) },
		"disable":      func() error { return cmdDisable(cmd, *serial) },
		"capabilities": cmdCapabilities,
		"test":         cmdTest,
		"camera":       func() error { return cmdCamera(cmd, *serial, fs.Arg(0)) },
		"zoom":         func() error { return cmdZoom(cmd, *serial, fs.Arg(0)) },
		"hq":           func() error { return cmdHighQuality(cmd, *serial) },
		"watch":        func() error { return cmdWatch(*serial) },
	}
	f, ok := cmds[cmd]
	if !ok {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err := f(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// selectDevice never picks arbitrarily when several devices are connected.
func selectDevice(cmd, serial string) (string, error) {
	devs, err := listDevices()
	if err != nil {
		return "", err
	}
	var ready []string
	unauthorized := false
	for _, d := range devs {
		if serial != "" && d.Serial == serial {
			if d.State != "device" {
				return "", fmt.Errorf("device %s is %s (accept the USB debugging prompt on the phone)", serial, d.State)
			}
			return serial, nil
		}
		if d.State == "device" {
			ready = append(ready, d.Serial)
		}
		unauthorized = unauthorized || d.State == "unauthorized"
	}
	switch {
	case serial != "":
		return "", fmt.Errorf("device %s not found", serial)
	case len(ready) == 1:
		return ready[0], nil
	case len(ready) > 1:
		return "", fmt.Errorf("Multiple Android devices detected.\n\nPlease specify a device:\n\nandroid-webcam %s --device %s", cmd, ready[0])
	case unauthorized:
		return "", errors.New("Android device is unauthorized.\n\nAccept the USB debugging authorization prompt on the phone.")
	}
	return "", errors.New(msgNoDevice)
}

func cmdDevices() error {
	devs, err := listDevices()
	if err != nil {
		return err
	}
	if len(devs) == 0 {
		return errors.New(msgNoDevice)
	}
	fmt.Print("Android devices:\n\n")
	for i, d := range devs {
		if d.State != "device" {
			fmt.Printf("%d. %s\n   State: %s\n\n", i+1, d.Serial, d.State)
			continue
		}
		fn, _ := usbFunctions(d.Serial)
		fmt.Printf("%d. %s\n   Serial: %s\n   Android: %s\n   USB: connected\n   UVC: %s\n   Webcam: %s\n\n",
			i+1, getprop(d.Serial, "ro.product.model"), d.Serial, getprop(d.Serial, "ro.build.version.release"),
			pick(supportsUvc(d.Serial, fn), "supported", "unsupported"), pick(fn == "uvc", "enabled", "disabled"))
	}
	return nil
}

func cmdStatus(cmd, serial string) error {
	d, err := selectDevice(cmd, serial)
	if err != nil {
		return err
	}
	fn, err := usbFunctions(d)
	if err != nil {
		return err
	}
	fmt.Printf("Device: %s\n\nAndroid:\n  Version: %s\n\nUSB:\n  Connected: yes\n  Function: %s\n\nWebcam:\n  Status: %s\n",
		getprop(d, "ro.product.model"), getprop(d, "ro.build.version.release"), fn, pick(fn == "uvc", "active", "inactive"))
	b := batteryInfo(d)
	fmt.Printf("\nBattery:\n  Level: %d%%\n  Temperature: %.1f°C%s\n  Charging: %s\n",
		b.level, b.tempC, pick(b.tempC >= hotC, " (hot)", ""), pick(b.charging, "yes", "no"))
	if c, err := findWebcam(); err != nil {
		fmt.Printf("\nHost camera:\n  Status: error (%v)\n", err)
	} else if c != nil {
		fmt.Printf("\nHost camera:\n  Name: %s\n  Status: detected\n", c.Name)
	} else {
		fmt.Print("\nHost camera:\n  Status: not detected\n")
	}
	return nil
}

func cmdDoctor(cmd, serial string) error {
	failed := false
	check := func(ok bool, msg, hint string) bool {
		if ok {
			fmt.Println("✓", msg)
			return true
		}
		fmt.Println("✗", msg)
		if hint != "" {
			fmt.Printf("\n%s\n", hint)
		}
		failed = true
		return false
	}
	fail := errors.New("Android Webcam is not ready.")

	fmt.Print("Android Webcam Diagnostics\n\nAndroid:\n")
	_, err := exec.LookPath("adb")
	if !check(err == nil, "ADB installed", "Install Android platform-tools and add adb to PATH.") {
		return fail
	}
	d, err := selectDevice(cmd, serial)
	if !check(err == nil, "ADB device detected and authorized", errText(err)) {
		return fail
	}
	sdk, _ := strconv.Atoi(getprop(d, "ro.build.version.sdk"))
	check(sdk >= 34, fmt.Sprintf("Android version supported (SDK %d, needs 34+)", sdk), "")
	fn, err := usbFunctions(d)
	if !check(err == nil, "USB function readable", errText(err)) {
		return fail
	}
	if !check(supportsUvc(d, fn), "UVC function available", msgUnsupported) {
		return fail
	}

	// Heat and battery degrade the stream but don't block it, so these only warn.
	if b := batteryInfo(d); b.tempC >= hotC {
		fmt.Printf("! Phone is hot (%.1f°C). Lower resolution, turn off High quality, or let it cool.\n", b.tempC)
	} else if !b.charging {
		fmt.Printf("! Phone is not charging (battery %d%%).\n", b.level)
	} else {
		check(true, fmt.Sprintf("Battery %d%%, %.1f°C, charging", b.level, b.tempC), "")
	}

	fmt.Print("\nUSB:\n")
	if fn != "uvc" {
		fmt.Printf("- Webcam mode off (current: %s). Run: android-webcam enable\n", fn)
	} else {
		check(true, "UVC function active", "")
		fmt.Print("\nHost:\n")
		c, err := findWebcam()
		check(err == nil && c != nil, "Camera detected by host", pick(err != nil, errText(err), msgHostMissing))
	}
	if failed {
		return fail
	}
	fmt.Println("\nReady to use Android as webcam.")
	return nil
}

func cmdEnable(cmd, serial string) error {
	d, err := selectDevice(cmd, serial)
	if err != nil {
		return err
	}
	fn, err := usbFunctions(d)
	if err != nil {
		return err
	}
	if !supportsUvc(d, fn) {
		return errors.New(msgUnsupported)
	}
	if fn != "uvc" {
		// Save before switching; never overwrite with "uvc" or disable couldn't restore.
		st, err := loadState()
		if err != nil {
			return err
		}
		st[d] = deviceState{PreviousUsbFunctions: fn, PreviousStayOn: stayOn(d)}
		if err := saveState(st); err != nil {
			return err
		}
		// Phone UI controls (camera/zoom/hq) need the screen on.
		if err := setStayOn(d, stayOnAny); err != nil {
			return err
		}
		if err := setUsbFunctions(d, "uvc"); err != nil {
			return err
		}
	}
	// Host enumeration lags the gadget switch by a few seconds.
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		if c, _ := findWebcam(); c != nil {
			fmt.Printf("✓ Android Webcam enabled\n\nCamera:\n  %s\n\nStatus:\n  Connected\n", c.Name)
			return nil
		}
	}
	return errors.New(msgHostMissing)
}

func cmdDisable(cmd, serial string) error {
	d, err := selectDevice(cmd, serial)
	if err != nil {
		return err
	}
	fn, err := usbFunctions(d)
	if err != nil {
		return err
	}
	st, err := loadState()
	if err != nil {
		return err
	}
	prev, saved := st[d]
	if saved && prev.PreviousStayOn != "" {
		if err := setStayOn(d, prev.PreviousStayOn); err != nil {
			return err
		}
	}
	if fn != "uvc" {
		// Device left webcam mode on its own (reboot, replug, other host): saved state is stale.
		if saved {
			delete(st, d)
			if err := saveState(st); err != nil {
				return err
			}
		}
		fmt.Printf("Webcam mode is not active (USB function: %s).\n", fn)
		return nil
	}
	if !saved {
		return fmt.Errorf("No saved USB configuration for %s (enabled from another host or manually).\n\nRestore manually, e.g.:\n\nadb -s %s shell svc usb setFunctions mtp", d, d)
	}
	if err := setUsbFunctions(d, prev.PreviousUsbFunctions); err != nil {
		return err
	}
	delete(st, d)
	if err := saveState(st); err != nil {
		return err
	}
	fmt.Printf("✓ Android Webcam disabled\n\nUSB restored to: %s\n", prev.PreviousUsbFunctions)
	return nil
}

// cmdWatch enables webcam once per plug-in, so a manual disable sticks until the cable is replugged.
func cmdWatch(serial string) error {
	log.Printf("Watching for Android devices%s (Ctrl+C to stop)", pick(serial != "", " ("+serial+")", ""))
	seen := map[string]bool{}
	for ; ; time.Sleep(2 * time.Second) {
		devs, err := listDevices()
		if err != nil {
			// adb server restarts and sleep/wake are transient; keep polling.
			log.Print(err)
			continue
		}
		now := map[string]bool{}
		for _, d := range devs {
			if d.State != "device" || (serial != "" && d.Serial != serial) {
				continue
			}
			now[d.Serial] = true
			if seen[d.Serial] {
				continue
			}
			// enable re-enumerates USB, so the device "reappears" already in uvc; skip that.
			if fn, err := usbFunctions(d.Serial); err != nil || fn == "uvc" {
				continue
			}
			log.Printf("%s connected, enabling webcam", d.Serial)
			if err := cmdEnable("watch", d.Serial); err != nil {
				log.Print(err)
			}
		}
		seen = now
	}
}

// ponytail: capabilities/test use the first Android Webcam on the host; --device not mapped to a host camera yet.
func detectedWebcam() (*camera, error) {
	c, err := findWebcam()
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, errors.New("Android Webcam not detected on host. Run: android-webcam enable")
	}
	return c, nil
}

func cmdCapabilities() error {
	c, err := detectedWebcam()
	if err != nil {
		return err
	}
	out, err := capabilities(*c)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n\n%s\n", c.Name, out)
	return nil
}

func cmdTest() error {
	c, err := detectedWebcam()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("ffplay"); err != nil {
		return errors.New("ffplay not found. Install FFmpeg to use the preview.")
	}
	p := exec.Command("ffplay", append([]string{"-hide_banner", "-window_title", c.Name}, previewArgs(*c)...)...)
	p.Stdin, p.Stdout, p.Stderr = os.Stdin, os.Stdout, os.Stderr
	return p.Run()
}

func pick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
