# PRD — Android as USB Webcam

## 1. Overview

### Product name

**Android Webcam**

### Problem

Developers/users often want to use an Android phone as a high-quality webcam for a macOS, Windows, or Linux computer.

Existing solutions commonly require:

- A dedicated Android webcam application
- A desktop companion application
- Wi-Fi/network streaming
- Third-party virtual camera drivers
- Additional accounts or subscriptions

These approaches introduce unnecessary latency, network dependencies, CPU usage, and setup complexity.

Modern Android versions can expose the device camera directly as a **USB Video Class (UVC) webcam**.

The goal of this project is to build a lightweight developer-oriented solution that manages and simplifies this native capability.

---

# 2. Goal

Provide a simple way to use an Android phone as a USB webcam.

Target experience:

```text
Connect Android phone via USB
        ↓
Enable Webcam mode
        ↓
Computer detects "Android Webcam"
        ↓
Use it in Zoom / Meet / Teams / OBS / etc.
```

The solution should prefer the **native Android UVC implementation** whenever supported.

---

# 3. Goals

### Primary goals

1. Detect whether a connected Android device supports USB Webcam/UVC mode.
2. Enable Android Webcam mode.
3. Disable Webcam mode and restore the previous USB configuration.
4. Verify that the host OS detects the phone as a webcam.
5. Provide a simple CLI/developer interface.
6. Support macOS, Windows, and Linux where the OS already supports UVC cameras.
7. Avoid network streaming whenever native UVC is available.

### Secondary goals

1. Provide camera capability information.
2. Provide diagnostics when webcam mode cannot be enabled.
3. Provide a simple preview/test command.
4. Support multiple connected Android devices.
5. Make the implementation suitable for CI/dev environments.

---

# 4. Non-goals

The first version will **not** implement:

- Custom video streaming over ADB
- RTSP streaming
- WebRTC streaming
- Cloud video streaming
- Remote webcam over Wi-Fi
- Video transcoding
- AI camera effects
- Background removal
- Face filters
- Camera recording
- Camera editing
- A custom virtual camera driver

The host should consume the Android phone as a standard UVC camera.

---

# 5. Target Architecture

```text
                    USB
┌────────────────┐        ┌──────────────────────┐
│ Android Phone  │        │ Host Computer        │
│                │        │                      │
│ Camera         │        │ macOS / Windows /    │
│     ↓          │        │ Linux                │
│ Android UVC    │───────→│                      │
│ Gadget         │        │ Native UVC Camera    │
└────────────────┘        └──────────┬───────────┘
                                     │
                                     ↓
                            Camera Applications
                          ┌───────────────────────┐
                          │ Zoom                  │
                          │ Google Meet           │
                          │ Microsoft Teams       │
                          │ OBS                   │
                          │ Browser               │
                          └───────────────────────┘
```

ADB is used only for **device control**, not as the video transport layer.

```text
ADB
 ↓
USB configuration
 ↓
UVC mode
```

Video:

```text
Android Camera
 ↓
USB UVC
 ↓
Host OS camera subsystem
```

---

# 6. Core Workflow

## 6.1 Detect devices

Command:

```bash
android-webcam devices
```

Expected output:

```text
Android devices:

1. Pixel 8
   Serial: ABC123
   Android: 16
   USB: connected
   UVC: supported
   Webcam: disabled
```

---

## 6.2 Check capability

Command:

```bash
android-webcam doctor
```

Example:

```text
Android Webcam Diagnostics

✓ ADB installed
✓ ADB device detected
✓ USB connection available
✓ Android version supported
✓ UVC function available
✓ Camera device available
✓ Host OS supports UVC

Ready to use Android as webcam.
```

Failure example:

```text
✗ UVC function is not available

The connected Android device does not expose
the USB Webcam capability.
```

---

# 7. Enable Webcam

Command:

```bash
android-webcam enable
```

Internally:

```bash
adb shell svc usb setFunctions uvc
```

Expected flow:

```text
Detect Android device
        ↓
Read current USB configuration
        ↓
Save current configuration
        ↓
Enable UVC
        ↓
Wait for USB reconnect
        ↓
Detect UVC camera on host
        ↓
Verify camera availability
```

Success:

```text
✓ Android Webcam enabled

Camera:
  Android Webcam

Status:
  Connected
```

---

# 8. Disable Webcam

Command:

```bash
android-webcam disable
```

The tool should restore the USB configuration that existed before Webcam mode.

Example:

```text
Before:

adb
mtp
```

After enabling:

```text
uvc
```

After disabling:

```text
adb
mtp
```

The tool must avoid blindly assuming the original configuration.

---

# 9. Status

Command:

```bash
android-webcam status
```

Example:

```text
Device: Pixel 8

Android:
  Version: 16

USB:
  Connected: yes
  Function: uvc

Webcam:
  Status: active

Host camera:
  Name: Android Webcam
  Status: detected

Capabilities:
  1920x1080 @ 30fps
  1280x720  @ 30fps
  640x480   @ 30fps
```

---

# 10. Camera Capability Discovery

The tool should be able to query the host camera capabilities.

On macOS, the implementation can use AVFoundation or FFmpeg during the diagnostic phase.

Example:

```bash
android-webcam capabilities
```

Output:

```text
Android Webcam

Resolution        FPS
----------------------
1920x1080         30
1280x720          30
1280x720          60
640x480           30
```

The exact implementation should be platform-specific.

---

# 11. Preview / Test

Command:

```bash
android-webcam test
```

The purpose is to verify:

1. Android camera is producing frames.
2. USB transport is working.
3. Host OS can consume the UVC stream.

Possible implementation:

```text
Android Webcam
      ↓
Host camera API
      ↓
Preview window
```

For the MVP, an existing host tool such as FFmpeg/FFplay can optionally be used instead of implementing a custom preview UI.

Example development test:

```bash
ffplay -f avfoundation -i "2"
```

---

# 12. Platform Support

## macOS

Primary host APIs:

- AVFoundation
- CoreMediaIO where required

Validation:

```bash
system_profiler SPCameraDataType
```

Example:

```text
Android Webcam:

  Model ID: UVC Camera VendorID_6353 ProductID_20206
```

FFmpeg validation:

```bash
ffmpeg -f avfoundation -list_devices true -i ""
```

Expected:

```text
[2] Android Webcam
```

---

## Linux

Primary host interface:

```text
V4L2
```

Validation:

```bash
v4l2-ctl --list-devices
```

Expected:

```text
Android Webcam
    /dev/video0
```

---

## Windows

Primary host interface:

- Media Foundation
- DirectShow compatibility where required

Expected:

```text
Camera:
  Android Webcam
```

---

# 13. CLI Design

The CLI should remain intentionally small.

```bash
android-webcam devices
android-webcam status
android-webcam doctor

android-webcam enable
android-webcam disable

android-webcam capabilities
android-webcam test
```

Optional:

```bash
android-webcam enable --device <serial>
android-webcam status --device <serial>
```

---

# 14. Multiple Devices

ADB may report multiple devices:

```bash
adb devices
```

Example:

```text
List of devices attached

ABC123    device
XYZ456    device
```

The CLI must not arbitrarily select a device.

If multiple devices exist:

```bash
android-webcam enable
```

should return:

```text
Multiple Android devices detected.

Please specify a device:

android-webcam enable --device ABC123
```

---

# 15. Device State Management

The tool must track:

```text
Original USB configuration
        ↓
Webcam mode enabled
        ↓
Webcam mode disabled
        ↓
Original USB configuration restored
```

State should preferably be stored locally on the host.

Example:

```text
~/.android-webcam/
    devices.json
```

Example state:

```json
{
  "ABC123": {
    "previousUsbFunctions": "adb,mtp"
  }
}
```

The implementation must consider:

- Device disconnect
- USB cable reconnect
- Computer sleep/wake
- Android reboot
- ADB restart
- Multiple hosts
- State becoming stale

---

# 16. Error Handling

## UVC unsupported

```text
Android Webcam is not supported on this device.

Possible reasons:
- Android version does not support USB Webcam
- Device manufacturer disabled UVC
- Device firmware does not expose the UVC function
```

## Device not connected

```text
No Android device detected.

Please:
1. Connect the device using USB.
2. Enable USB debugging.
3. Accept the debugging authorization prompt.
```

## UVC enabled but host cannot detect camera

```text
UVC mode was enabled, but the host OS did not detect
the Android camera.

Please check:
- USB cable
- USB connection mode
- Camera permissions
- Host camera services
```

## Multiple devices

Require explicit device selection.

---

# 17. Security

ADB requires USB debugging authorization.

The tool must:

- Never bypass Android ADB authorization.
- Never install an APK without explicit user action.
- Never upload camera frames to a remote server.
- Never transmit camera data over the network by default.
- Keep all configuration/state local.

Camera frames should remain inside:

```text
Android → USB → Host
```

No cloud infrastructure is required.

---

# 18. Privacy

The product should be privacy-first.

### No network required

The core webcam functionality must work completely offline.

### No video storage

The tool must not automatically record camera frames.

### No analytics

MVP should not require telemetry.

---

# 19. Performance Requirements

The native UVC path should introduce minimal additional latency.

Target:

```text
Camera → USB → Host
```

rather than:

```text
Camera
 ↓
H264 encoding
 ↓
Network/ADB
 ↓
Decoding
 ↓
Virtual camera
```

Target requirements:

| Metric | Target |
|---|---:|
| Additional latency | < 100 ms |
| 1080p | Supported where device exposes it |
| 30 FPS | Required where device supports it |
| CPU usage on host | Minimal |
| Network | Not required |
| Video storage | None |

Actual performance should be measured across representative Android devices.

---

# 20. Compatibility

The product should distinguish between:

### Supported

Android exposes:

```text
uvc
```

and host detects:

```text
Android Webcam
```

### Unsupported

Android does not expose:

```text
uvc
```

### Partially supported

Android exposes UVC but:

- Camera cannot start
- Unsupported resolution
- Unsupported FPS
- Host cannot enumerate camera

The diagnostic command should clearly identify which layer failed.

---

# 21. Diagnostics Architecture

Diagnostics should be divided into three layers.

```text
Layer 1
Android / ADB
       ↓
Can we control the device?

Layer 2
USB / UVC
       ↓
Does Android expose a webcam?

Layer 3
Host Camera API
       ↓
Can macOS / Windows / Linux consume it?
```

Example:

```text
Android:
✓ Device connected
✓ ADB authorized
✓ UVC function available

USB:
✓ UVC interface detected

Host:
✓ Camera detected
✓ Camera opened successfully
✓ Frames received

Result:
✓ Android Webcam is ready
```

---

# 22. MVP

The first implementation should only contain:

### Required

- [ ] Detect Android devices
- [ ] Select device
- [ ] Detect UVC capability
- [ ] Save current USB function
- [ ] Enable UVC
- [ ] Disable UVC
- [ ] Restore previous USB function
- [ ] Detect host webcam
- [ ] Basic diagnostics
- [ ] macOS support
- [ ] Linux support
- [ ] Windows support

### Optional

- [ ] Camera capability discovery
- [ ] Preview
- [ ] Multiple-device management
- [ ] GUI

---

# 23. Future Versions

## V2

- GUI
- Automatic device detection
- Automatic UVC activation
- Camera resolution selection
- FPS selection
- Device nickname
- Tray/menu-bar application

## V3

Potential fallback for devices without native UVC:

```text
Android Camera
      ↓
H264
      ↓
ADB
      ↓
Desktop decoder
      ↓
Virtual Camera
```

This fallback should only be implemented if there is a real need.

---

# 24. Recommended Technical Direction

The core principle is:

> **Do not reinvent the webcam transport if Android already provides UVC.**

Recommended architecture:

```text
                 ┌──────────────────┐
                 │ Android Device   │
                 │                  │
                 │ Camera           │
                 │      ↓           │
                 │ Native UVC       │
                 └────────┬─────────┘
                          │ USB
                          ▼
                 ┌──────────────────┐
                 │ Host             │
                 │                  │
                 │ OS UVC subsystem │
                 └────────┬─────────┘
                          │
             ┌────────────┼────────────┐
             ▼            ▼            ▼
           Zoom         Meet          OBS
```

ADB exists only beside the video pipeline:

```text
ADB ─────────────→ USB configuration/control

UVC ─────────────→ Video
```

This separation keeps the architecture simple, low-latency, and platform-native.

---

# 25. Success Criteria

The project is considered successful when a user can perform:

```bash
adb devices

android-webcam enable
```

and then open:

```text
Zoom / Google Meet / Teams / OBS
```

and select:

```text
Android Webcam
```

with no additional Android application, network connection, account, or proprietary webcam driver required.

For the tested macOS environment, the expected validation is:

```bash
system_profiler SPCameraDataType
```

showing:

```text
Android Webcam
```

and:

```bash
ffmpeg -f avfoundation -list_devices true -i ""
```

showing:

```text
[2] Android Webcam
```

This confirms the Android device is successfully exposed to macOS as a native UVC camera.
