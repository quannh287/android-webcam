package main

import "strings"

type camera struct{ Name, ID, Path string }

// Match by Google VID/PID (18d1:4eee) too, since the camera name may differ per OEM/OS.
func isAndroidWebcam(c camera) bool {
	id := strings.ToLower(c.ID)
	return strings.Contains(c.Name, "Android Webcam") ||
		strings.Contains(c.ID, "VendorID_6353 ProductID_20206") ||
		(strings.Contains(id, "18d1") && strings.Contains(id, "4eee"))
}

func findWebcam() (*camera, error) {
	cams, err := hostCameras()
	if err != nil {
		return nil, err
	}
	for _, c := range cams {
		if isAndroidWebcam(c) {
			return &c, nil
		}
	}
	return nil, nil
}
