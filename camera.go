package main

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reActiveCam = regexp.MustCompile(`Camera ID: (\d+),[^)]*Client Package Name: ` + regexp.QuoteMeta(webcamPackage))
	reHalDevice = regexp.MustCompile(`Camera HAL device \S+/(\d+) .*static information`)
	reFacing    = regexp.MustCompile(`^\s*Facing: (\w+)`)
	reBounds    = regexp.MustCompile(`bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"`)
)

// webcamFacing parses `dumpsys media.camera`; ok=false when the webcam service has no camera open (host not streaming).
func webcamFacing(dump string) (facing string, ok bool) {
	m := reActiveCam.FindStringSubmatch(dump)
	if m == nil {
		return "", false
	}
	id := m[1]
	cur := ""
	for _, line := range strings.Split(dump, "\n") {
		if h := reHalDevice.FindStringSubmatch(line); h != nil {
			cur = h[1]
		} else if f := reFacing.FindStringSubmatch(line); f != nil && cur == id {
			return strings.ToLower(f[1]), true
		}
	}
	// ponytail: HAL dump format varies by OEM; fall back to the Android convention 0=back, 1=front.
	return map[string]string{"0": "back", "1": "front"}[id], true
}

func currentFacing(serial string) (string, bool) {
	dump, _ := shell(serial, "dumpsys", "media.camera")
	return webcamFacing(dump)
}

// nodeByID returns the uiautomator <node> tag for a DeviceAsWebcam view id, or "".
func nodeByID(ui, id string) string {
	return regexp.MustCompile(`<node [^>]*resource-id="` + regexp.QuoteMeta(webcamPackage+":id/"+id) + `"[^>]*>`).FindString(ui)
}

func nodeBounds(node string) (x1, y1, x2, y2 int, ok bool) {
	b := reBounds.FindStringSubmatch(node)
	if b == nil {
		return 0, 0, 0, 0, false
	}
	fmt.Sscan(b[1]+" "+b[2]+" "+b[3]+" "+b[4], &x1, &y1, &x2, &y2)
	return x1, y1, x2, y2, true
}

func nodeAttr(node, name string) string {
	m := regexp.MustCompile(` ` + name + `="([^"]*)"`).FindStringSubmatch(node)
	if m == nil {
		return ""
	}
	return m[1]
}

type zoomOption struct {
	ratio float64
	node  string
}

// zoomOptions reads the preset buttons ("0,5" / "1,0" / "2,0", locale decimal) and which one is selected.
func zoomOptions(ui string) (opts []zoomOption, selected int) {
	selected = -1
	sx1, _, sx2, _, hasSel := nodeBounds(nodeByID(ui, "zoom_ui_toggle_btn_selected"))
	for _, id := range []string{"zoom_ui_toggle_option_low", "zoom_ui_toggle_option_middle", "zoom_ui_toggle_option_high"} {
		node := nodeByID(ui, id)
		r, err := strconv.ParseFloat(strings.Replace(nodeAttr(node, "text"), ",", ".", 1), 64)
		if node == "" || err != nil {
			continue
		}
		// The highlight is a separate view sitting on top of the selected option.
		if x1, _, x2, _, ok := nodeBounds(node); ok && hasSel && (sx1+sx2)/2 >= x1 && (sx1+sx2)/2 <= x2 {
			selected = len(opts)
		}
		opts = append(opts, zoomOption{r, node})
	}
	return opts, selected
}

func dumpUI(serial string) string {
	ui, _ := shell(serial, "uiautomator dump /sdcard/android-webcam-ui.xml >/dev/null && cat /sdcard/android-webcam-ui.xml")
	return ui
}

func tapNode(serial, node string) error {
	x1, y1, x2, y2, ok := nodeBounds(node)
	if !ok {
		return errors.New("control has no bounds")
	}
	_, err := shell(serial, "input", "tap", fmt.Sprint((x1+x2)/2), fmt.Sprint((y1+y2)/2))
	return err
}

// requireStreaming returns the device only when webcam mode is on and a host app is consuming the stream.
func requireStreaming(cmd, serial string) (string, string, error) {
	d, err := selectDevice(cmd, serial)
	if err != nil {
		return "", "", err
	}
	if fn, _ := usbFunctions(d); fn != "uvc" {
		return "", "", errors.New("Webcam mode is not active. Run: android-webcam enable")
	}
	facing, ok := currentFacing(d)
	if !ok {
		return "", "", errors.New("Webcam is not streaming. Open the camera in a host app (Zoom/Meet/OBS) first.")
	}
	return d, facing, nil
}

// ponytail: DeviceAsWebcam exposes no API for its controls, so we drive its preview UI; breaks if AOSP renames view ids.
// openPreview returns the preview UI dump once its controls accept taps; always pair with closePreview.
func openPreview(serial string) (string, error) {
	shell(serial, "input", "keyevent", "KEYCODE_WAKEUP")
	if win, _ := shell(serial, "dumpsys", "window"); strings.Contains(win, "isKeyguardShowing=true") {
		return "", errors.New("Phone is locked. Unlock the screen and try again.")
	}
	if _, err := shell(serial, "am", "start", "-n", webcamPackage+"/com.android.deviceaswebcam.DeviceAsWebcamPreview"); err != nil {
		return "", err
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		if nodeByID(dumpUI(serial), "toggle_camera_button") != "" {
			// Taps are ignored while the freshly opened preview initializes.
			time.Sleep(1500 * time.Millisecond)
			return dumpUI(serial), nil
		}
	}
	return "", errors.New("Webcam preview controls not found. Unlock the phone screen and try again.")
}

func closePreview(serial string) { shell(serial, "input", "keyevent", "KEYCODE_BACK") }

func cmdCamera(cmd, serial, want string) error {
	if want != "" && want != "front" && want != "back" {
		return errors.New("Usage: android-webcam camera [front|back] [--device <serial>]")
	}
	d, cur, err := requireStreaming(cmd, serial)
	if err != nil {
		return err
	}
	if want == "" || want == cur {
		fmt.Printf("Camera: %s\n", cur)
		return nil
	}
	ui, err := openPreview(d)
	defer closePreview(d)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := tapNode(d, nodeByID(ui, "toggle_camera_button")); err != nil {
			return err
		}
		for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
			if cur, _ = currentFacing(d); cur == want {
				fmt.Printf("✓ Camera: %s\n", cur)
				return nil
			}
		}
	}
	return fmt.Errorf("camera did not switch to %s (still %s)", want, cur)
}

func cmdZoom(cmd, serial, arg string) error {
	want, err := strconv.ParseFloat(strings.Replace(arg, ",", ".", 1), 64)
	if arg != "" && err != nil {
		return errors.New("Usage: android-webcam zoom [ratio, e.g. 0.5 | 1 | 2] [--device <serial>]")
	}
	d, _, err := requireStreaming(cmd, serial)
	if err != nil {
		return err
	}
	ui, err := openPreview(d)
	defer closePreview(d)
	if err != nil {
		return err
	}
	opts, sel := zoomOptions(ui)
	if len(opts) == 0 {
		return errors.New("Zoom controls not available (High quality mode may hide them).")
	}
	if arg == "" {
		for i, o := range opts {
			fmt.Printf("%s %gx\n", pick(i == sel, "*", " "), o.ratio)
		}
		return nil
	}
	// Presets differ per lens (e.g. front 0.9/1/2), so pick the closest one.
	best := 0
	for i, o := range opts {
		if math.Abs(o.ratio-want) < math.Abs(opts[best].ratio-want) {
			best = i
		}
	}
	if best != sel {
		if err := tapNode(d, opts[best].node); err != nil {
			return err
		}
		time.Sleep(time.Second)
		if _, sel = zoomOptions(dumpUI(d)); sel != best {
			return fmt.Errorf("zoom did not switch to %gx", opts[best].ratio)
		}
	}
	fmt.Printf("✓ Zoom: %gx\n", opts[best].ratio)
	return nil
}

// ponytail: HQ state is only exposed as a localized label, so this toggles and echoes the new label.
func cmdHighQuality(cmd, serial string) error {
	d, _, err := requireStreaming(cmd, serial)
	if err != nil {
		return err
	}
	ui, err := openPreview(d)
	defer closePreview(d)
	if err != nil {
		return err
	}
	if err := tapNode(d, nodeByID(ui, "high_quality_button")); err != nil {
		return err
	}
	time.Sleep(time.Second)
	fmt.Printf("✓ High quality toggled (button now: %q)\n", nodeAttr(nodeByID(dumpUI(d), "high_quality_button"), "content-desc"))
	return nil
}
