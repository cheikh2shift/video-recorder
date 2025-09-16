import cv2
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
