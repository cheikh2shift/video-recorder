import sys
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
