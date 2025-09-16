import sys
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
