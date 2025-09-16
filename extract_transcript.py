import sys
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