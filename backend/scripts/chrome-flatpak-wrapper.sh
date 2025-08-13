#!/bin/bash
# Chrome Flatpak wrapper script for chromedp

# Grant network access and disable sandbox for automation
exec flatpak run --share=network --socket=x11 --filesystem=home com.google.Chrome \
    --no-first-run \
    --no-default-browser-check \
    --disable-default-apps \
    --disable-popup-blocking \
    --disable-translate \
    --disable-features=TranslateUI \
    --disable-ipc-flooding-protection \
    "$@"