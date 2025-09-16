let screenStream, webcamStream, mediaRecorderScreen, mediaRecorderWebcam;
let wsScreen, wsWebcam;
let recordedScreenChunks = [], recordedWebcamChunks = [];
let sessionId = null;

const screenVideo = document.getElementById('screenVideo');
const webcamVideo = document.getElementById('webcamVideo');
const startBtn = document.getElementById('startBtn');
const stopBtn = document.getElementById('stopBtn');
const statusDiv = document.getElementById('status');
const finalVideo = document.getElementById('finalVideo');
const timerSpan = document.getElementById('timer');
let timerInterval = null;
let timerStart = null;

let allRecords = [];
let transcriptMap = {};
let currentPage = 1;
const recordsPerPage = 6;

function renderRecords(records, page = 1) {
  const tbody = document.querySelector('#recordsTable tbody');
  tbody.innerHTML = '';
  const start = (page - 1) * recordsPerPage;
  const paginated = records.slice(start, start + recordsPerPage);
  paginated.forEach(rec => {
    transcriptMap[rec.session] = rec.transcript || "";
    const tr = document.createElement('tr');
    tr.innerHTML = `<td>${new Date(rec.date).toLocaleString()}</td><td>${rec.duration.toFixed(2)}</td><td>${rec.session}</td><td><button class='btn btn-sm btn-primary play-btn' data-session='${rec.session}'>Play</button></td>`;
    tbody.appendChild(tr);
  });
  // Add play button event listeners
  document.querySelectorAll('.play-btn').forEach(btn => {
    btn.onclick = function() {
      const session = btn.getAttribute('data-session');
      const transcript = transcriptMap[session] || "";
      showVideoModal(`http://localhost:8080/uploads/${session}_final.mp4`, transcript);
    };
  });
  renderPagination(records.length, page);
}

function renderPagination(total, page) {
  const pagination = document.getElementById('pagination');
  pagination.innerHTML = '';
  const totalPages = Math.ceil(total / recordsPerPage);
  if (totalPages <= 1) return;
  for (let i = 1; i <= totalPages; i++) {
    const li = document.createElement('li');
    li.className = 'page-item' + (i === page ? ' active' : '');
    const a = document.createElement('a');
    a.className = 'page-link';
    a.textContent = i;
    a.onclick = () => {
      currentPage = i;
      renderRecords(filteredRecords(), currentPage);
    };
    li.appendChild(a);
    pagination.appendChild(li);
  }
}

function filteredRecords() {
  const searchDate = document.getElementById('searchDate').value;
  const searchId = document.getElementById('searchId').value.trim();
  let filtered = allRecords;
  if (searchDate) {
    filtered = filtered.filter(rec => {
      const recDate = new Date(rec.date).toISOString().slice(0, 10);
      return recDate === searchDate;
    });
  }
  if (searchId) {
    filtered = filtered.filter(rec => rec.session.includes(searchId));
  }
  return filtered;
}

function loadRecords() {
  fetch('http://localhost:8080/records')
    .then(r => r.json())
    .then(records => {
      allRecords = records;
      currentPage = 1;
      renderRecords(filteredRecords(), currentPage);
    });
}

document.addEventListener('DOMContentLoaded', () => {
  loadRecords();
  document.getElementById('searchDate').addEventListener('input', () => {
    currentPage = 1;
    renderRecords(filteredRecords(), currentPage);
  });
  document.getElementById('searchId').addEventListener('input', () => {
    currentPage = 1;
    renderRecords(filteredRecords(), currentPage);
  });
});

window.addEventListener('DOMContentLoaded', loadRecords);

startBtn.onclick = async () => {
  startBtn.disabled = true;
  stopBtn.disabled = false;
  statusDiv.textContent = 'Initializing...';
  timerSpan.textContent = '00:00';
  timerStart = Date.now();
  timerInterval = setInterval(() => {
    const elapsed = Math.floor((Date.now() - timerStart) / 1000);
    const min = String(Math.floor(elapsed / 60)).padStart(2, '0');
    const sec = String(elapsed % 60).padStart(2, '0');
    timerSpan.textContent = `${min}:${sec}`;
  }, 500);

  // Generate a unique session ID
  sessionId = ([1e7]+-1e3+-4e3+-8e3+-1e11).replace(/[018]/g, c =>
    (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16)
  );

  // Get anime filter state
  const animeFilter = document.getElementById('animeFilter').checked ? '1' : '0';

  // Get screen and webcam streams
  screenStream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: true });
  webcamStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });

  screenVideo.srcObject = screenStream;

  webcamVideo.style.display = '';
  webcamVideo.srcObject = webcamStream;

  // Connect to backend WebSocket with session ID and anime filter
  wsScreen = new WebSocket(`ws://localhost:8080/ws/screen?session=${sessionId}`);
  wsWebcam = new WebSocket(`ws://localhost:8080/ws/webcam?session=${sessionId}&anime=${animeFilter}`);

  // Prefer MP4, fallback to WebM
  let screenMime = 'video/mp4';
  let webcamMime = 'video/mp4';
  if (!MediaRecorder.isTypeSupported(screenMime)) screenMime = 'video/webm; codecs=vp8';
  if (!MediaRecorder.isTypeSupported(webcamMime)) webcamMime = 'video/webm; codecs=vp8';

  // Record screen
  mediaRecorderScreen = new MediaRecorder(screenStream, { mimeType: screenMime });
  mediaRecorderScreen.ondataavailable = e => {
    if (e.data.size > 0) {
      wsScreen.readyState === 1 && wsScreen.send(e.data);
      recordedScreenChunks.push(e.data);
    }
  };
  // Ensure all remaining chunks are sent on stop
  mediaRecorderScreen.onstop = () => {
    recordedScreenChunks.forEach(chunk => {
      if (wsScreen && wsScreen.readyState === 1) {
        wsScreen.send(chunk);
      }
    });
    wsScreen.close();
  };
  mediaRecorderScreen.start(50);

  // Record webcam
  mediaRecorderWebcam = new MediaRecorder(webcamStream, { mimeType: webcamMime });
  mediaRecorderWebcam.ondataavailable = e => {
    if (e.data.size > 0) {
      wsWebcam.readyState === 1 && wsWebcam.send(e.data);
      recordedWebcamChunks.push(e.data);
    }
  };
  // Ensure all remaining chunks are sent on stop
  mediaRecorderWebcam.onstop = () => {
    recordedWebcamChunks.forEach(chunk => {
      if (wsWebcam && wsWebcam.readyState === 1) {
        wsWebcam.send(chunk);
      }
    });
    wsWebcam.close();
  };
  mediaRecorderWebcam.start(50);

  statusDiv.textContent = 'Recording...';
};

stopBtn.onclick = () => {
  startBtn.disabled = false;
  stopBtn.disabled = true;
  statusDiv.textContent = 'Processing...';
  if (timerInterval) {
    clearInterval(timerInterval);
    timerInterval = null;
  }
  timerSpan.textContent = '';

  if (mediaRecorderScreen) mediaRecorderScreen.stop();
  if (mediaRecorderWebcam) mediaRecorderWebcam.stop();

  // Turn off webcam after recording
  if (webcamStream) {
    webcamStream.getTracks().forEach(track => track.stop());
    webcamVideo.srcObject = null;
  }

  // Show loading overlay
  showLoadingOverlay();

  // Wait 10 seconds before making backend call
  setTimeout(() => {
    // Combine chunks for preview
    const screenBlob = new Blob(recordedScreenChunks, { type: 'video/webm' });
    const webcamBlob = new Blob(recordedWebcamChunks, { type: 'video/webm' });
    // Show previews
    screenVideo.srcObject = null;
    webcamVideo.srcObject = null;
    screenVideo.src = URL.createObjectURL(screenBlob);
    webcamVideo.src = URL.createObjectURL(webcamBlob);
    // Revert to WebSocket approach: close sockets after all chunks sent
    if (wsScreen && wsScreen.readyState === 1) wsScreen.close();
    if (wsWebcam && wsWebcam.readyState === 1) wsWebcam.close();
    // Request final video from backend
    const animeFilter = document.getElementById('animeFilter').checked ? '1' : '0';
    const ttsOverlay = document.getElementById('ttsOverlay').checked ? '1' : '0';
    startSSE(sessionId);
    fetch(`http://localhost:8080/final?session=${sessionId}&anime=${animeFilter}&ttsOverlay=${ttsOverlay}`).then(async r => {
      if (r.ok) {
        const blob = await r.blob();
        const url = URL.createObjectURL(blob);
        statusDiv.textContent = 'Final video ready!';
        hideLoadingOverlay();
        loadRecords();
        showVideoModal(url);
      } else {
        statusDiv.textContent = 'Error processing video.';
        hideLoadingOverlay();
      }
    });
  }, 10000);
};

function showVideoModal(url, transcript = "") {
  const modalVideo = document.getElementById('modalVideo');
  modalVideo.src = url;
  const modalTranscript = document.getElementById('modalTranscript');
  modalTranscript.textContent = transcript;
  // Show modal using Bootstrap
  const modal = new bootstrap.Modal(document.getElementById('videoModal'));
  modal.show();
}

function showLoadingOverlay() {
  let overlay = document.getElementById('loadingOverlay');
  if (!overlay) {
    overlay = document.createElement('div');
    overlay.id = 'loadingOverlay';
    overlay.style.position = 'fixed';
    overlay.style.top = '0';
    overlay.style.left = '0';
    overlay.style.width = '100vw';
    overlay.style.height = '100vh';
    overlay.style.background = 'rgba(0,0,0,0.5)';
    overlay.style.zIndex = '9999';
    overlay.style.display = 'flex';
    overlay.style.alignItems = 'center';
    overlay.style.justifyContent = 'center';
    overlay.innerHTML = `<div class='spinner-border text-light' style='width:4rem;height:4rem;' role='status'></div><span class='ms-4 text-light fs-3' id='loadingStep'>Processing video...</span>`;
    document.body.appendChild(overlay);
  } else {
    overlay.style.display = 'flex';
  }
}
function hideLoadingOverlay() {
  const overlay = document.getElementById('loadingOverlay');
  if (overlay) overlay.style.display = 'none';
}

function startSSE(sessionId) {
  const source = new EventSource(`http://localhost:8080/progress?session=${sessionId}`);
  source.onmessage = function(event) {
    const data = event.data;
    const stepElem = document.getElementById('loadingStep');
    if (stepElem) stepElem.textContent = data;
    if (data === 'done') {
      source.close();
    }
  };
}
