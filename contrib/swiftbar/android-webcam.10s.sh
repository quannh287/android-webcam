#!/bin/bash
# <swiftbar.title>Android Webcam</swiftbar.title>
# <swiftbar.hideRunInTerminal>true</swiftbar.hideRunInTerminal>
# <swiftbar.hideLastUpdated>true</swiftbar.hideLastUpdated>
# Menu-bar wrapper around the android-webcam CLI. Set ANDROID_WEBCAM if the binary isn't on PATH.

# SwiftBar runs with a minimal PATH; adb usually lives in Homebrew.
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/Library/Android/sdk/platform-tools:$PATH"
AW="$(command -v "${ANDROID_WEBCAM:-android-webcam}")"

if [ -z "$AW" ]; then
  echo "| sfimage=video.slash"
  echo "---"
  echo "android-webcam not found (set ANDROID_WEBCAM)"
  exit 0
fi

item() { echo "$1 | bash=$AW $2 terminal=false refresh=true"; }

status="$("$AW" status 2>&1)"
if grep -q "Status: active" <<<"$status"; then
  facing="$("$AW" camera 2>/dev/null | awk '{print $NF}')"
  echo "${facing:-on} | sfimage=video.fill"
  echo "---"
  grep -E "^Device:|Temperature:" <<<"$status" | sed 's/^ *//'
  echo "---"
  item "Front camera" "param1=camera param2=front"
  item "Back camera" "param1=camera param2=back"
  echo "Zoom"
  for z in 0.5 1 2; do item "--${z}x" "param1=zoom param2=$z"; done
  item "Toggle High quality" "param1=hq"
  echo "---"
  item "Disable webcam" "param1=disable"
elif grep -q "^Device:" <<<"$status"; then
  echo "| sfimage=video.slash"
  echo "---"
  grep "^Device:" <<<"$status"
  item "Enable webcam" "param1=enable"
else
  echo "| sfimage=video.slash"
  echo "---"
  head -1 <<<"$status"
fi
echo "---"
echo "Run doctor | bash=$AW param1=doctor terminal=true"
