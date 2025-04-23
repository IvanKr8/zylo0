package log

import "os"

func Create(path string) error {
	_, err := os.Create(path)
	if err != nil {
		return err
	}
	return nil
}

func Read(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}
