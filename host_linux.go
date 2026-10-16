package main

import (
	"os"
	"path/filepath"
	"strings"
)

func hostCameras() ([]camera, error) {
	nodes, err := filepath.Glob("/sys/class/video4linux/video*")
	if err != nil {
		return nil, err
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return strings.TrimSpace(string(b)) }
	var cams []camera
	for _, n := range nodes {
		// device -> USB interface; its parent holds the USB device's VID/PID.
		usb := filepath.Join(n, "device", "..")
		cams = append(cams, camera{
			Name: read(filepath.Join(n, "name")),
			ID:   read(filepath.Join(usb, "idVendor")) + ":" + read(filepath.Join(usb, "idProduct")),
			// ponytail: first matching node is usually the capture node, not UVC metadata
			Path: "/dev/" + filepath.Base(n),
		})
	}
	return cams, nil
}

func capabilities(c camera) (string, error) {
	return run("v4l2-ctl", "--list-formats-ext", "-d", c.Path)
}

func previewArgs(c camera) []string {
	return []string{"-f", "v4l2", "-i", c.Path}
}
