package heaterstore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const PendingValueFilename = ".pendingvalue"

type Store struct {
	Dir string
}

func (h *Store) Get(username, id string) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join(h.Dir, username, id))
	if err != nil {
		return "", err
	}
	return strings.Trim(string(data), "\n"), nil
}

func (h *Store) Set(username, id, value string) error {
	current, err := h.Get(username, id)
	if err != nil {
		return err
	}

	if current != value {
		err = ioutil.WriteFile(filepath.Join(h.Dir, username, id), []byte(value), 0644)
		if err != nil {
			return err
		}
	}
	return nil
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
