#!/bin/bash
# Double-click this file in Finder before each run of the day. It puts both copies of the app
# back on the same machine and rewinds the clock, then tells you when to press Play.
cd "$(dirname "$0")" || exit 1
export PATH="/opt/homebrew/opt/helm@3/bin:/opt/homebrew/bin:$PATH"
echo "Setting the stage. This takes about a minute."
if ./deploy/scene.sh; then
  echo
  echo "Ready. Go to http://localhost:35590 and press \"Play the day\"."
else
  echo
  echo "Something went wrong. Check that OrbStack is open, wait a minute and try again."
fi
echo "You can close this window."
