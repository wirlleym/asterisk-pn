package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// DeviceRecord e o mapeamento ramal <-> token de push.
type DeviceRecord struct {
	Extension string `json:"extension"`
	Platform  string `json:"platform"`
	Provider  string `json:"provider"`
	Token     string `json:"token"`
	DeviceID  string `json:"deviceId"`
	UpdatedAt string `json:"updatedAt"`
}

// Store persiste os devices num arquivo JSON (sem banco, sem dependencia).
type Store struct {
	mu      sync.Mutex
	path    string
	devices map[string]DeviceRecord
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path, devices: map[string]DeviceRecord{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s.devices)
	}
	return s, nil
}

func (s *Store) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.devices, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Upsert(rec DeviceRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[rec.DeviceID] = rec
	return s.persist()
}

func (s *Store) FindByExtension(ext string) []DeviceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []DeviceRecord{}
	for _, rec := range s.devices {
		if rec.Extension == ext {
			out = append(out, rec)
		}
	}
	return out
}
