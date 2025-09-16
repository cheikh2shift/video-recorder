# Video Recording Backend Setup

## About

This project is a full-featured video recording and processing backend for browser-based webcam and screen recording. It supports real-time streaming to a Go server, advanced post-processing (cartoon/anime filter, noise reduction, transcript extraction, and TTS overlay), and session-based file management. The system uses Python scripts for video and audio processing, Whisper for transcription, and Edge TTS for high-quality voice synthesis. All recordings and metadata are tracked in a local SQLite database, and the UI allows for easy preview, search, and download of processed videos.

## Quickstart

1. Ensure you have Python 3.8+ and Go installed.
2. Ensure you have `ffmpeg` installed and available in your system PATH. This is required for all video/audio processing steps. On Ubuntu/Debian: `sudo apt install ffmpeg`
3. Run the setup:
   go run main.go --setup
   (This will write all required Python scripts, create a Python virtual environment in ./venv, and install all Python dependencies.)
4. Build and install the Go server binary:
   go install
   (This will place the binary in your Go bin directory, e.g., `~/go/bin/video-call-backend`)
5. To launch the server:
   video-call-backend
   (Or use the full path to the binary if not in your PATH)

## Python dependencies
- opencv-python
- noisereduce
- soundfile
- numpy
- whisper
- edge-tts

All scripts will be written to the backend directory if missing.

## System Dependencies
- **ffmpeg**: Required for all video and audio processing. Install via your system package manager (e.g., `sudo apt install ffmpeg` on Ubuntu/Debian, `brew install ffmpeg` on macOS).

## Features
- Browser-based webcam and screen recording
- Real-time streaming to Go backend via WebSocket
- MP4 conversion and merging
- Optional anime/cartoon filter (OpenCV)
- Audio noise reduction
- Transcript extraction (Whisper)
- TTS overlay (Edge TTS)
- Session-based file management and SQLite record tracking
- Modern Bootstrap UI with preview, search, and download
