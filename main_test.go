package main

import (
	"reflect"
	"testing"
)

func TestParseAdbDevices(t *testing.T) {
	out := "* daemon started successfully\nList of devices attached\nABC123\tdevice\nXYZ456\tunauthorized\n\n"
	want := []adbDevice{{"ABC123", "device"}, {"XYZ456", "unauthorized"}}
	if got := parseAdbDevices(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestIsAndroidWebcam(t *testing.T) {
	cases := map[camera]bool{
		{Name: "Android Webcam"}: true,
		{Name: "Cam", ID: "UVC Camera VendorID_6353 ProductID_20206 0x13200018d14eee"}: true,
		{Name: "USB Video", ID: `USB\VID_18D1&PID_4EEE&MI_00\6&1`}:                     true,
		{Name: "UVC", ID: "18d1:4eee"}:                                                 true,
		{Name: "FaceTime HD Camera", ID: "FaceTime HD Camera 46435DF4"}:                false,
	}
	for c, want := range cases {
		if got := isAndroidWebcam(c); got != want {
			t.Errorf("%+v: got %v, want %v", c, got, want)
		}
	}
}

func TestWebcamFacing(t *testing.T) {
	dump := `(Camera ID: 1, Cost: 50, PID: 29565, Score: 0, State: 2User Id: 0, Client Package Name: com.android.DeviceAsWebcam, Conflicting Client Devices: {7, 8, })
== Camera HAL device device@1.1/internal/0 (v1.4) static information: ==
    Facing: Back
== Camera HAL device device@1.1/internal/1 (v1.4) static information: ==
    Facing: Front`
	if f, ok := webcamFacing(dump); !ok || f != "front" {
		t.Fatalf("got %q %v, want front", f, ok)
	}
	if _, ok := webcamFacing("Active Camera Clients:\n[]"); ok {
		t.Fatal("want not streaming")
	}
}

func TestNodeByID(t *testing.T) {
	ui := `<node index="0" resource-id="com.android.DeviceAsWebcam:id/high_quality_button" content-desc="Turn off High quality mode" bounds="[82,181][154,253]" /><node index="1" text="" resource-id="com.android.DeviceAsWebcam:id/toggle_camera_button" clickable="true" bounds="[664,1975][808,2119]" />`
	if x1, y1, x2, y2, ok := nodeBounds(nodeByID(ui, "toggle_camera_button")); !ok || x1 != 664 || y1 != 1975 || x2 != 808 || y2 != 2119 {
		t.Fatalf("got %d,%d,%d,%d %v", x1, y1, x2, y2, ok)
	}
	if got := nodeAttr(nodeByID(ui, "high_quality_button"), "content-desc"); got != "Turn off High quality mode" {
		t.Fatalf("got %q", got)
	}
}

func TestZoomOptions(t *testing.T) {
	ui := `<node text="0,9" resource-id="com.android.DeviceAsWebcam:id/zoom_ui_toggle_option_low" bounds="[240,2006][312,2089]" />
<node text="1,0" resource-id="com.android.DeviceAsWebcam:id/zoom_ui_toggle_option_middle" bounds="[323,2005][395,2089]" />
<node text="2,0" resource-id="com.android.DeviceAsWebcam:id/zoom_ui_toggle_option_high" bounds="[407,2005][479,2089]" />
<node text="" resource-id="com.android.DeviceAsWebcam:id/zoom_ui_toggle_btn_selected" bounds="[324,2011][396,2083]" />`
	opts, sel := zoomOptions(ui)
	if len(opts) != 3 || opts[0].ratio != 0.9 || opts[2].ratio != 2 || sel != 1 {
		t.Fatalf("got %+v sel=%d", opts, sel)
	}
}

func TestParseBattery(t *testing.T) {
	b := parseBattery("Current Battery Service state:\n  AC powered: true\n  USB powered: false\n  level: 87\n  temperature: 412\n")
	if b.level != 87 || b.tempC != 41.2 || !b.charging {
		t.Fatalf("got %+v", b)
	}
}

func TestStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	st, err := loadState()
	if err != nil || len(st) != 0 {
		t.Fatalf("empty load: %v %v", st, err)
	}
	st["ABC123"] = deviceState{PreviousUsbFunctions: "mtp"}
	if err := saveState(st); err != nil {
		t.Fatal(err)
	}
	got, err := loadState()
	if err != nil || got["ABC123"].PreviousUsbFunctions != "mtp" {
		t.Fatalf("got %v %v", got, err)
	}
}
