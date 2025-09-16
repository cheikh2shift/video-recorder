package main

// This file contains the source code for all required Python scripts as string constants.

const cartoonizePy = `import cv2
import sys

def cartoonize_video(input_path, output_path):
    cap = cv2.VideoCapture(input_path)
    fourcc = cv2.VideoWriter_fourcc(*'mp4v')
    fps = cap.get(cv2.CAP_PROP_FPS)
    w = int(cap.get(cv2.CAP_PROP_FRAME_WIDTH))
    h = int(cap.get(cv2.CAP_PROP_FRAME_HEIGHT))
    out = cv2.VideoWriter(output_path, fourcc, fps, (w, h))
    while True:
        ret, frame = cap.read()
        if not ret:
            break
        cartoon = cv2.stylization(frame, sigma_s=150, sigma_r=0.25)
        out.write(cartoon)
    cap.release()
    out.release()

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: python cartoonize.py input.mp4 output.mp4")
        sys.exit(1)
    cartoonize_video(sys.argv[1], sys.argv[2])
`

const enhanceMicPy = `import sys
import os
import subprocess
import noisereduce as nr
import soundfile as sf
import numpy as np

def reduce_noise(input_wav, output_wav):
    data, rate = sf.read(input_wav)
    # If stereo, use only one channel for noise profile
    if len(data.shape) > 1:
        noise_clip = data[:,0]
    else:
        noise_clip = data
    reduced = nr.reduce_noise(y=data, sr=rate, y_noise=noise_clip)
    sf.write(output_wav, reduced, rate)

def main():
    if len(sys.argv) != 3:
        print("Usage: python3 enhance_mic.py <input_audio> <output_audio>")
        sys.exit(1)
    input_audio = sys.argv[1]
    output_audio = sys.argv[2]

    # Convert input to wav if needed using ffmpeg
    wav_audio = input_audio
    if not input_audio.endswith('.wav'):
        wav_audio = input_audio + '.tmp.wav'
        cmd = [
            'ffmpeg', '-y', '-i', input_audio, wav_audio
        ]
        result = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if result.returncode != 0:
            print("ffmpeg conversion failed:", result.stderr.decode())
            sys.exit(1)

    reduce_noise(wav_audio, output_audio)

    # Clean up temp wav file
    if wav_audio != input_audio:
        os.remove(wav_audio)

if __name__ == "__main__":
    main()
`

const extractTranscriptPy = `import sys
import whisper
import re
import difflib

if len(sys.argv) != 3:
    print("Usage: python3 extract_transcript.py <input_video> <output_txt>")
    sys.exit(1)
input_video = sys.argv[1]
output_txt = sys.argv[2]
model = whisper.load_model("medium")
result = model.transcribe(input_video)
segments = result["segments"] if "segments" in result else []

# Segment-level fuzzy deduplication
def deduplicate_segments_fuzzy(segments, threshold=0.9):
    deduped = []
    seen = []
    for seg in segments:
        text = seg["text"].strip()
        if not any(difflib.SequenceMatcher(None, text, s).ratio() > threshold for s in seen):
            deduped.append(seg)
            seen.append(text)
    return deduped

segments = deduplicate_segments_fuzzy(segments)
text = ' '.join(seg["text"].strip() for seg in segments)

# Deduplicate repeated sentences/phrases
def deduplicate_text(text):
    # Split into sentences using punctuation
    sentences = re.split(r'(?<=[.!?])\s+', text.strip())
    seen = set()
    deduped = []
    for sentence in sentences:
        s = sentence.strip()
        if s and s not in seen:
            deduped.append(s)
            seen.add(s)
    return ' '.join(deduped)

deduped_text = deduplicate_text(text)
with open(output_txt, "w") as f:
    f.write(deduped_text)
`

const overlayTtsPy = `import sys
import os
from whisper import load_model
import tempfile
import asyncio
import edge_tts
import re
import difflib

def save_tts(text, out_path, voice="en-US-GuyNeural"):
    if not text or not text.strip():
        # Generate a short silence if text is empty
        os.system(f"ffmpeg -f lavfi -i anullsrc=r=16000:cl=mono -t 0.3 -acodec pcm_s16le '{out_path}'")
        return
    asyncio.run(edge_tts.Communicate(text, voice).save(out_path))

def deduplicate_segments(segments):
    seen = set()
    deduped = []
    for seg in segments:
        text = seg["text"].strip()
        if text and text not in seen:
            deduped.append(seg)
            seen.add(text)
    return deduped

def deduplicate_segments_fuzzy(segments, threshold=0.9):
    deduped = []
    seen = []
    for seg in segments:
        text = seg["text"].strip()
        # Fuzzy match against previous segments
        if not any(difflib.SequenceMatcher(None, text, s).ratio() > threshold for s in seen):
            deduped.append(seg)
            seen.append(text)
    return deduped

def overlay_tts_on_audio(video_path, output_path, model_size="medium"):
    # Extract audio from video
    audio_path = tempfile.mktemp(suffix=".wav")
    os.system(f"ffmpeg -y -i '{video_path}' -vn -acodec pcm_s16le -ar 16000 -ac 1 '{audio_path}'")

    # Transcribe with Whisper
    model = load_model(model_size)
    result = model.transcribe(audio_path, word_timestamps=True)
    segments = result["segments"]

    # Fuzzy deduplicate segment texts
    segments = deduplicate_segments_fuzzy(segments)

    # Generate TTS for each segment and concatenate with silence using ffmpeg concat
    tts_files = []
    concat_list_path = tempfile.mktemp(suffix=".txt")
    with open(concat_list_path, "w") as concat_list:
        for i, seg in enumerate(segments):
            tts_path = tempfile.mktemp(suffix=".wav")
            save_tts(seg["text"], tts_path)
            tts_files.append(tts_path)
            concat_list.write(f"file '{tts_path}'\n")
            # Add silence between segments if pause detected
            if i < len(segments) - 1:
                pause = segments[i+1]["start"] - seg["end"]
                # Use the actual pause duration, even for short pauses
                if pause > 0.05:  # Insert silence for any pause > 50ms
                    silence_path = tempfile.mktemp(suffix=".wav")
                    os.system(f"ffmpeg -f lavfi -i anullsrc=r=16000:cl=mono -t {pause} -acodec pcm_s16le '{silence_path}'")
                    tts_files.append(silence_path)
                    concat_list.write(f"file '{silence_path}'\n")

    tts_concat_path = tempfile.mktemp(suffix="_tts_concat.wav")
    # Concatenate all TTS and silence files
    os.system(f"ffmpeg -y -f concat -safe 0 -i '{concat_list_path}' -c copy '{tts_concat_path}'")

    # Export only the TTS audio (no original audio)
    os.system(f"ffmpeg -y -i '{tts_concat_path}' -c:a libmp3lame '{output_path}'")

    # Cleanup temp files
    os.remove(audio_path)
    os.remove(concat_list_path)
    os.remove(tts_concat_path)
    for f in tts_files:
        if os.path.exists(f):
            os.remove(f)

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python overlay_tts.py <video_path> <output_path>")
        sys.exit(1)
    overlay_tts_on_audio(sys.argv[1], sys.argv[2])
`
