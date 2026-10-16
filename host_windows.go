package main

import "encoding/json"

func hostCameras() ([]camera, error) {
	// -InputObject @() keeps a JSON array even when there is a single camera.
	out, err := run("powershell", "-NoProfile", "-Command",
		"ConvertTo-Json -InputObject @(Get-PnpDevice -Class Camera,Image -PresentOnly | Select-Object FriendlyName,InstanceId)")
	if err != nil {
		return nil, err
	}
	var items []struct{ FriendlyName, InstanceId string }
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, err
	}
	var cams []camera
	for _, i := range items {
		cams = append(cams, camera{Name: i.FriendlyName, ID: i.InstanceId})
	}
	return cams, nil
}

func capabilities(c camera) (string, error) {
	// ffmpeg exits non-zero after listing options, so the output is what matters.
	out, _ := run("ffmpeg", "-hide_banner", "-f", "dshow", "-list_options", "true", "-i", "video="+c.Name)
	return out, nil
}

func previewArgs(c camera) []string {
	return []string{"-f", "dshow", "-i", "video=" + c.Name}
}
