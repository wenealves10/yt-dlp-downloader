package utils

import (
	"fmt"
	"os"
)

func FileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return !info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func CreateFolder(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return nil
		}
		return fmt.Errorf("o caminho '%s' já existe e não é uma pasta", path)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("erro ao verificar a existência da pasta: %w", err)
	}

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return fmt.Errorf("erro ao criar pasta '%s': %w", path, err)
	}

	return nil
}

func RemoveFile(path string) error {
	if exists, _ := FileExists(path); exists {
		err := os.Remove(path)
		if err != nil {
			return err
		}
	}
	return nil
}
