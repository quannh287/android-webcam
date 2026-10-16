package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func hostCameras() ([]camera, error) {
	out, err := run("system_profiler", "SPCameraDataType", "-json")
	if err != nil {
		return nil, err
	}
	var r struct {
		Items []struct {
			Name   string `json:"_name"`
			Model  string `json:"spcamera_model-id"`
			Unique string `json:"spcamera_unique-id"`
		} `json:"SPCameraDataType"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return nil, err
	}
	var cams []camera
	for _, i := range r.Items {
		cams = append(cams, camera{Name: i.Name, ID: i.Model + " " + i.Unique})
	}
	return cams, nil
}

func capabilities(c camera) (string, error) {
	// ponytail: avfoundation only prints supported modes when an invalid size is requested
	out, _ := run("ffmpeg", "-hide_banner", "-f", "avfoundation", "-video_size", "1x1", "-i", c.Name)
	return parseAvfoundationModes(out), nil
}

// Lines look like "[avfoundation @ 0x..]   1920x1080@[30.000030 30.000030]fps"; repeated per pixel format.
func parseAvfoundationModes(out string) string {
	var b strings.Builder
	seen := map[string]bool{}
	fmt.Fprintf(&b, "%-18s%s\n%s\n", "Resolution", "FPS", strings.Repeat("-", 22))
	for _, line := range strings.Split(out, "\n") {
		var fps float64
		f := strings.Fields(line)
		if len(f) == 0 || !strings.HasSuffix(line, "fps") {
			continue
		}
		res, rate, ok := strings.Cut(f[len(f)-2], "@[")
		if !ok {
			continue
		}
		if _, err := fmt.Sscan(rate, &fps); err != nil {
			continue
		}
		row := fmt.Sprintf("%-18s%.0f", res, fps)
		if !seen[row] {
			seen[row] = true
			b.WriteString(row + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func previewArgs(c camera) []string {
	return []string{"-f", "avfoundation", "-framerate", "30", "-i", c.Name}
}
