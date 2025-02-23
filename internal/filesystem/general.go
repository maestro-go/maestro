package filesystem

import (
	"errors"
	"os"
)

func CheckFSObject(fsPath string) (bool, error) {
	_, err := os.Stat(fsPath)

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, nil
	}
	return true, nil
}
