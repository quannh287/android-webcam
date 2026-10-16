# android-webcam

Turn an Android 14+ phone into a native USB webcam (UVC) from the command line — no phone app, no host driver, no network.

## Requirements

- **Android 14+ with DeviceAsWebcam** — the built-in USB webcam feature (`com.android.DeviceAsWebcam`); some manufacturers disable it.
- **adb** on `PATH`, USB debugging enabled and the host authorized.
- **Go 1.27+** to build (stdlib only, no dependencies).
- **Optional:** `ffmpeg`/`ffplay` for `capabilities` and `test`; `v4l2-ctl` on Linux.

## Install

```sh
git clone https://github.com/quannh287/android-webcam && cd android-webcam
go build -o android-webcam .   # or: go install .
```

## Commands

| Command | What it does |
| --- | --- |
| `devices` | List connected Android devices |
| `status` | Show webcam status (USB function, host camera, temperature) |
| `doctor` | Diagnose the Android / USB / host layers |
| `enable` | Switch the device to USB webcam (UVC) mode |
| `disable` | Restore the USB configuration saved by `enable` |
| `capabilities` | List resolutions/FPS exposed to the host |
| `test` | Open a preview window (requires `ffplay`) |
| `camera [front\|back]` | Show or switch the streaming camera |
| `zoom [ratio]` | Show zoom presets or pick the closest, e.g. `zoom 0.5` |
| `hq` | Toggle the phone's High quality mode |
| `watch` | Enable webcam automatically whenever a device is plugged in |

- **`--device <serial>`** selects a device when several are connected.

## How it works

- **adb only controls USB config** — `enable` runs `svc usb setFunctions uvc`; video then flows over the native UVC class, so any host app (Zoom, Meet, OBS, QuickTime) sees a normal camera.
- **State is saved** in `~/.android-webcam/devices.json` (previous USB function and stay-awake setting), so `disable` restores exactly what was there.
- **`camera`, `zoom`, `hq` drive the DeviceAsWebcam preview UI** via `uiautomator`. The phone must be unlocked and a host app must be streaming, otherwise the preview screen is not shown. `enable` keeps the screen awake while plugged in for this reason.
- **`watch`** enables once per plug-in, so a manual `disable` sticks until the cable is replugged.

## macOS extras

- **Menu bar (SwiftBar)** — copy or symlink `contrib/swiftbar/android-webcam.10s.sh` into your SwiftBar plugin folder. It shows status/temperature and offers enable/disable, front/back camera, zoom and HQ toggle. Set `ANDROID_WEBCAM` if the binary is not on `PATH`.
- **Auto-enable at login (launchd)** — edit the binary path in `contrib/macos/com.android-webcam.watch.plist`, then:

```sh
cp contrib/macos/com.android-webcam.watch.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.android-webcam.watch.plist
```

Logs go to `/tmp/android-webcam-watch.log`.

## Known limitations

- **Tested on** Pixel 8 Pro (Android 17) with a macOS host only.
- **Linux/Windows host detection** compiles but is untested on real hardware.
- **Image is landscape** — hold the phone horizontally.
- **ADB briefly drops** while USB re-enumerates during the switch; the CLI waits for it to come back.
- **`capabilities`/`test`** use the first Android webcam found on the host; `--device` is not mapped to a host camera yet.

See [docs/PRD.md](docs/PRD.md) for the full design.
