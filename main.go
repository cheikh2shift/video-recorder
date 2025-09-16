package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
)

var (
	mu sync.Mutex
	db *sql.DB
)

// SSE progress tracking
var progressMap = make(map[string]chan string)

func progressHandler(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	if session == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	ch := make(chan string, 10)
	progressMap[session] = ch
	defer delete(progressMap, session)
	for msg := range ch {
		fmt.Fprintf(w, "data: %s\n\n", msg)
		flusher.Flush()
		if msg == "done" {
			break
		}
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func wsHandler(fileType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.URL.Query().Get("session")
		if session == "" {
			http.Error(w, "Missing session ID", http.StatusBadRequest)
			return
		}
		filePath := fmt.Sprintf("uploads/%s_%s.webm", session, fileType)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println("Upgrade error:", err)
			return
		}
		defer conn.Close()
		f, err := os.Create(filePath)
		if err != nil {
			fmt.Println("File error:", err)
			return
		}
		defer f.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			mu.Lock()
			f.Write(data)
			mu.Unlock()
		}
	}
}

func recordsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, session, date, duration, transcript FROM records ORDER BY date DESC")
	if err != nil {
		http.Error(w, "DB error", 500)
		return
	}
	defer rows.Close()
	type Record struct {
		ID         int     `json:"id"`
		Session    string  `json:"session"`
		Date       string  `json:"date"`
		Duration   float64 `json:"duration"`
		Transcript string  `json:"transcript"`
	}
	var records []Record
	for rows.Next() {
		var rec Record
		rows.Scan(&rec.ID, &rec.Session, &rec.Date, &rec.Duration, &rec.Transcript)
		records = append(records, rec)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

func finalHandler(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	anime := r.URL.Query().Get("anime")
	if session == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}
	screenWebm := fmt.Sprintf("uploads/%s_screen.webm", session)
	webcamWebm := fmt.Sprintf("uploads/%s_webcam.webm", session)
	screenFile := fmt.Sprintf("uploads/%s_screen.mp4", session)
	webcamFile := fmt.Sprintf("uploads/%s_webcam.mp4", session)
	finalFile := fmt.Sprintf("uploads/%s_final.mp4", session)

	// SSE progress: screen conversion
	if ch, ok := progressMap[session]; ok {
		ch <- "Converting screen recording..."
	}
	cmdScreen := exec.Command("ffmpeg", "-i", screenWebm, "-c:v", "libx264", "-c:a", "aac", "-y", screenFile)
	fmt.Printf("Running command: ffmpeg -i %s -c:v libx264 -c:a aac -y %s\n", screenWebm, screenFile)
	screenErr, _ := cmdScreen.CombinedOutput()
	if cmdScreen.ProcessState == nil || !cmdScreen.ProcessState.Success() {
		fmt.Printf("ffmpeg screen conversion error: %s\n", string(screenErr))
	}

	if ch, ok := progressMap[session]; ok {
		ch <- "Converting webcam recording..."
	}
	cmdWebcam := exec.Command("ffmpeg", "-i", webcamWebm, "-c:v", "libx264", "-c:a", "aac", "-y", webcamFile)
	fmt.Printf("Running command: ffmpeg -i %s -c:v libx264 -c:a aac -y %s\n", webcamWebm, webcamFile)
	webcamErr, _ := cmdWebcam.CombinedOutput()
	if cmdWebcam.ProcessState == nil || !cmdWebcam.ProcessState.Success() {
		fmt.Printf("ffmpeg webcam conversion error: %s\n", string(webcamErr))
	}

	if anime == "1" {
		if ch, ok := progressMap[session]; ok {
			ch <- "Applying cartoon filter..."
		}
		cartoonWebcamFile := fmt.Sprintf("uploads/%s_webcam_cartoon.mp4", session)
		cmdCartoon := exec.Command("python3", "cartoonize.py", webcamFile, cartoonWebcamFile)
		fmt.Printf("Running command: python3 cartoonize.py %s %s\n", webcamFile, cartoonWebcamFile)
		stderrPipe, _ := cmdCartoon.StderrPipe()
		if err := cmdCartoon.Start(); err != nil {
			w.WriteHeader(500)
			w.Write([]byte("cartoonize error: failed to start"))
			return
		}
		errOutput, _ := io.ReadAll(stderrPipe)
		err := cmdCartoon.Wait()
		if err != nil {
			fmt.Printf("cartoonize.py stderr: %s\n", string(errOutput))
			w.WriteHeader(500)
			w.Write([]byte("cartoonize error: " + string(errOutput)))
			return
		}
		if len(errOutput) > 0 {
			fmt.Printf("cartoonize.py stderr: %s\n", string(errOutput))
		}
		if ch, ok := progressMap[session]; ok {
			ch <- "Extracting and enhancing webcam audio..."
		}
		audioFile := fmt.Sprintf("uploads/%s_webcam_audio.aac", session)
		cmdAudio := exec.Command("ffmpeg", "-i", webcamFile, "-vn", "-acodec", "copy", "-y", audioFile)
		fmt.Printf("Running command: ffmpeg -i %s -vn -acodec copy -y %s\n", webcamFile, audioFile)
		audioErr, _ := cmdAudio.CombinedOutput()
		if cmdAudio.ProcessState == nil || !cmdAudio.ProcessState.Success() {
			fmt.Printf("ffmpeg audio extraction error: %s\n", string(audioErr))
		}
		// Enhance audio using Python script
		enhancedAudio := fmt.Sprintf("uploads/%s_webcam_audio_enhanced.mp3", session)
		cmdEnhance := exec.Command("python3", "enhance_mic.py", audioFile, enhancedAudio)
		fmt.Printf("Running command: python3 enhance_mic.py %s %s\n", audioFile, enhancedAudio)
		enhanceErr, _ := cmdEnhance.CombinedOutput()
		if cmdEnhance.ProcessState == nil || !cmdEnhance.ProcessState.Success() {
			fmt.Printf("enhance_mic.py error: %s\n", string(enhanceErr))
		}
		if ch, ok := progressMap[session]; ok {
			ch <- "Merging cartoon video and enhanced audio..."
		}
		cartoonWithAudio := fmt.Sprintf("uploads/%s_webcam_cartoon_audio.mp4", session)
		cmdMerge := exec.Command("ffmpeg", "-i", cartoonWebcamFile, "-i", enhancedAudio, "-c:v", "copy", "-c:a", "aac", "-y", cartoonWithAudio)
		fmt.Printf("Running command: ffmpeg -i %s -i %s -c:v copy -c:a aac -y %s\n", cartoonWebcamFile, enhancedAudio, cartoonWithAudio)
		mergeErr, _ := cmdMerge.CombinedOutput()
		if cmdMerge.ProcessState == nil || !cmdMerge.ProcessState.Success() {
			fmt.Printf("ffmpeg audio merge error: %s\n", string(mergeErr))
		}
		webcamFile = cartoonWithAudio
	}
	if ch, ok := progressMap[session]; ok {
		ch <- "Composing final video..."
	}
	filter := "[1:v][0:v]scale2ref=iw*0.2:ih*0.2[v1][ref];[v1]pad=iw+8:ih+8:4:4:color=blue[v2];[ref][v2]overlay=W-w-20:20:shortest=1"
	cmd := exec.Command("ffmpeg",
		"-i", screenFile,
		"-i", webcamFile,
		"-filter_complex", filter,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-y", finalFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		w.WriteHeader(500)
		w.Write([]byte("ffmpeg error"))
		return
	}
	if ch, ok := progressMap[session]; ok {
		ch <- "Extracting video duration..."
	}
	probeCmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", finalFile)
	out, err := probeCmd.Output()
	var duration float64
	if err == nil {
		fmt.Sscanf(string(out), "%f", &duration)
	}
	if ch, ok := progressMap[session]; ok {
		ch <- "Extracting transcript..."
	}
	transcript := ""
	transcriptFile := fmt.Sprintf("uploads/%s_transcript.txt", session)
	cmdTranscript := exec.Command("python3", "extract_transcript.py", finalFile, transcriptFile)
	fmt.Printf("Running command: python3 extract_transcript.py %s %s\n", finalFile, transcriptFile)
	transcriptErr, _ := cmdTranscript.CombinedOutput()
	if cmdTranscript.ProcessState != nil && cmdTranscript.ProcessState.Success() {
		data, err := os.ReadFile(transcriptFile)
		if err == nil {
			transcript = string(data)
		}
	} else {
		fmt.Printf("Whisper transcript error: %s\n", string(transcriptErr))
	}

	ttsOverlay := r.URL.Query().Get("ttsOverlay")
	if ttsOverlay == "1" {
		if ch, ok := progressMap[session]; ok {
			ch <- "Generating TTS overlay..."
		}
		ttsAudioFile := fmt.Sprintf("uploads/%s_tts_overlay.mp3", session)
		cmdTTS := exec.Command("python3", "overlay_tts.py", finalFile, ttsAudioFile)
		fmt.Printf("Running command: python3 overlay_tts.py %s %s\n", finalFile, ttsAudioFile)
		ttsErr, _ := cmdTTS.CombinedOutput()
		if cmdTTS.ProcessState == nil || !cmdTTS.ProcessState.Success() {
			fmt.Printf("overlay_tts.py error: %s\n", string(ttsErr))
		}
		if ch, ok := progressMap[session]; ok {
			ch <- "Replacing audio with TTS..."
		}
		tmpFile := fmt.Sprintf("uploads/%s_final_tmp.mp4", session)
		cmdReplaceTTS := exec.Command("ffmpeg", "-i", finalFile, "-i", ttsAudioFile, "-c:v", "copy", "-map", "0:v:0", "-map", "1:a:0", "-shortest", "-y", tmpFile)
		fmt.Printf("Running command: ffmpeg -i %s -i %s -c:v copy -map 0:v:0 -map 1:a:0 -shortest -y %s\n", finalFile, ttsAudioFile, tmpFile)
		replaceTTSErr, _ := cmdReplaceTTS.CombinedOutput()
		if cmdReplaceTTS.ProcessState == nil || !cmdReplaceTTS.ProcessState.Success() {
			fmt.Printf("ffmpeg TTS audio replace error: %s\n", string(replaceTTSErr))
		}
		// Move tmpFile to finalFile
		os.Rename(tmpFile, finalFile)
	}
	finalWithTTS := finalFile

	if ch, ok := progressMap[session]; ok {
		ch <- "Saving record..."
	}
	db.Exec("INSERT INTO records (session, date, duration, transcript) VALUES (?, ?, ?, ?)", session, time.Now().Format(time.RFC3339), duration, transcript)
	if ch, ok := progressMap[session]; ok {
		ch <- "done"
	}
	f, err := os.Open(finalWithTTS)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("file error"))
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "video/mp4")
	io.Copy(w, f)
}

func writeFileIfNotExists(path, content string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(content)
		return err
	}
	return nil
}

func setupPythonEnv() error {
	// Write scripts
	scripts := []struct{ name, content string }{
		{"cartoonize.py", cartoonizePy},
		{"enhance_mic.py", enhanceMicPy},
		{"extract_transcript.py", extractTranscriptPy},
		{"overlay_tts.py", overlayTtsPy},
	}
	for _, s := range scripts {
		err := writeFileIfNotExists(s.name, s.content)
		if err != nil {
			return fmt.Errorf("failed to write %s: %v", s.name, err)
		}
	}

	// Setup venv and install deps
	if _, err := os.Stat("venv"); os.IsNotExist(err) {
		cmd := exec.Command("python3", "-m", "venv", "venv")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("venv creation failed: %v", err)
		}
	}
	cmd := exec.Command("venv/bin/pip", "install", "--upgrade", "pip")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Run()
	cmd = exec.Command("venv/bin/pip", "install", "-r", "requirements.txt")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pip install failed: %v", err)
	}
	return nil
}

func main() {
	setupFlag := flag.Bool("setup", false, "Write scripts, set up Python venv, install dependencies, and create README")
	flag.Parse()
	if *setupFlag {
		err := setupPythonEnv()
		if err != nil {
			log.Fatalf("Setup failed: %v", err)
		}

		fmt.Println("Setup complete. See README.md for instructions.")
		return
	}
	var err error
	db, err = sql.Open("sqlite3", "uploads/records.db")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session TEXT,
		date TEXT,
		duration REAL,
		transcript TEXT
	)`)
	if err != nil {
		panic(err)
	}
	// Ensure uploads directory exists
	uploadsDir := "uploads"
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		err := os.MkdirAll(uploadsDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create uploads directory: %v", err)
		}
	}
	http.Handle("/ws/screen", wsHandler("screen"))
	http.Handle("/ws/webcam", wsHandler("webcam"))
	http.HandleFunc("/final", finalHandler)
	http.HandleFunc("/progress", progressHandler)
	http.HandleFunc("/records", recordsHandler)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	http.Handle("/", http.FileServer(http.Dir("frontend")))
	fmt.Println("Server started at :8080")
	http.ListenAndServe(":8080", nil)
}
