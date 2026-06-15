package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type Handler struct {
	store *Store
	hub   *Hub
}

type createClipReq struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

func (h *Handler) CreateClip(w http.ResponseWriter, r *http.Request) {
	var req createClipReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Type != "text" {
		http.Error(w, "type must be 'text'", http.StatusBadRequest)
		return
	}

	clip := &Clip{
		ID:        uuid.New().String(),
		Type:      "text",
		Content:   req.Content,
		CreatedAt: time.Now(),
	}
	if err := h.store.CreateClip(clip); err != nil {
		log.Println("CreateClip:", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.hub.Broadcast(clip)
	writeJSON(w, http.StatusCreated, clip)
}

func (h *Handler) GetLatestClip(w http.ResponseWriter, r *http.Request) {
	clip, err := h.store.GetLatestClip()
	if err != nil {
		http.Error(w, "no clips", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, clip)
}

func (h *Handler) GetClip(w http.ResponseWriter, r *http.Request) {
	clip, err := h.store.GetClip(r.PathValue("id"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, clip)
}

func (h *Handler) ListClips(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	clips, err := h.store.ListClips(limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if clips == nil {
		clips = []Clip{}
	}
	writeJSON(w, http.StatusOK, clips)
}

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Println("UploadFile formfile:", err)
		http.Error(w, "no file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	id := uuid.New().String()
	ext := filepath.Ext(header.Filename)
	savedName := id + ext

	dst, err := os.Create(filepath.Join("uploads", savedName))
	if err != nil {
		log.Println("create file:", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(filepath.Join("uploads", savedName))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	clip := &Clip{
		ID:        id,
		Type:      "file",
		Filename:  header.Filename,
		FileSize:  written,
		CreatedAt: time.Now(),
	}
	if err := h.store.CreateClip(clip); err != nil {
		os.Remove(filepath.Join("uploads", savedName))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.hub.Broadcast(clip)
	writeJSON(w, http.StatusCreated, clip)
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	clip, err := h.store.GetClip(r.PathValue("id"))
	if err != nil || clip.Type != "file" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ext := filepath.Ext(clip.Filename)
	filePath := filepath.Join("uploads", clip.ID+ext)

	w.Header().Set("Content-Disposition", "attachment; filename=\""+clip.Filename+"\"")
	http.ServeFile(w, r, filePath)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
