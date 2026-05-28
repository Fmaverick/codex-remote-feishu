package daemon

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

type projectActivityStore struct {
	mu      sync.Mutex
	path    string
	entries []control.ProjectActivityEntry
}

func newProjectActivityStore(path string) *projectActivityStore {
	store := &projectActivityStore{path: strings.TrimSpace(path)}
	store.load()
	return store
}

func (s *projectActivityStore) Append(entry control.ProjectActivityEntry) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	if len(s.entries) > 5000 {
		s.entries = append([]control.ProjectActivityEntry(nil), s.entries[len(s.entries)-5000:]...)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		log.Printf("project activity mkdir failed: path=%s err=%v", s.path, err)
		return
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("project activity open failed: path=%s err=%v", s.path, err)
		return
	}
	defer file.Close()
	payload, err := json.Marshal(entry)
	if err != nil {
		log.Printf("project activity marshal failed: err=%v", err)
		return
	}
	if _, err := file.Write(append(payload, '\n')); err != nil {
		log.Printf("project activity append failed: path=%s err=%v", s.path, err)
	}
}

func (s *projectActivityStore) Recent(surfaceID string, limit int) []control.ProjectActivityEntry {
	if s == nil || limit <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	surfaceID = strings.TrimSpace(surfaceID)
	out := make([]control.ProjectActivityEntry, 0, limit)
	for i := len(s.entries) - 1; i >= 0 && len(out) < limit; i-- {
		entry := s.entries[i]
		if surfaceID != "" && strings.TrimSpace(entry.SurfaceSessionID) != surfaceID {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (s *projectActivityStore) load() {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return
	}
	file, err := os.Open(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("project activity load failed: path=%s err=%v", s.path, err)
		}
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry control.ProjectActivityEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if strings.TrimSpace(entry.SurfaceSessionID) == "" {
			continue
		}
		s.entries = append(s.entries, entry)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("project activity scan failed: path=%s err=%v", s.path, err)
	}
	if len(s.entries) > 5000 {
		s.entries = append([]control.ProjectActivityEntry(nil), s.entries[len(s.entries)-5000:]...)
	}
}
