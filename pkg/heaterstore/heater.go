package heaterstore

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const PendingValueFilename = ".pendingvalue"

type Store struct {
	sync.Mutex
	Dir string
}

type Record struct {
	Value   string
	Version int
}

func (h *Store) Get(username, id string) (Record, error) {
	r := Record{}
	data, err := ioutil.ReadFile(filepath.Join(h.Dir, username, id))
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(data, &r)
	if err != nil {
		return r, err
	}
	return r, nil
}

func (h *Store) Set(username, id, value string) (Record, error) {
	h.Lock()
	defer h.Unlock()
	r, err := h.Get(username, id)
	if err != nil {
		return r, err
	}
	r.Version++
	data, err := json.Marshal(r)
	if err != nil {
		return r, err
	}

	err = ioutil.WriteFile(filepath.Join(h.Dir, username, id), data, 0644)
	if err != nil {
		return r, err
	}
	return r, nil
}

func (h *Store) IDs(username string) ([]string, error) {
	files, err := ioutil.ReadDir(filepath.Join(h.Dir, username))
	if err != nil {
		return []string{}, err
	}
	ids := []string{}
	for _, file := range files {
		if file.Name() != PendingValueFilename {
			ids = append(ids, file.Name())
		}
	}
	return ids, nil
}

func (h *Store) UserExists(username string) bool {
	fileinfo, err := os.Stat(filepath.Join(h.Dir, username))
	return !(os.IsNotExist(err) || fileinfo.IsDir() != true)
}

func (h *Store) SetPendingValue(username, value string) error {
	return ioutil.WriteFile(filepath.Join(h.Dir, username, PendingValueFilename), []byte(value), 0644)
}

func (h *Store) GetPendingValue(username string) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join(h.Dir, username, PendingValueFilename))
	if err != nil {
		return "", err
	}
	return strings.Trim(string(data), "\n"), nil
}

func (h *Store) DelPendingValue(username string) error {
	err := os.Remove(filepath.Join(h.Dir, username, PendingValueFilename))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
