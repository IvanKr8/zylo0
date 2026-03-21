package container

import (
	"fmt"
	"time"
	namee "zylo/name"
)

func checkName(name, containerID string) error {
	var n string

	if name != "" {
		ok, err := namee.NameExists(name)
		if err != nil {
			return fmt.Errorf("failed to check name existence: %v", err)
		}
		if ok {
			return fmt.Errorf("name '%s' already exists", name)
		}
		n = name
	} else {
		n = generateContainerName()
	}

	if err := namee.RegisterName(n, containerID); err != nil {
		return fmt.Errorf("failed to register name: %v", err)
	}

	return nil
}

func generateContainerName() string {
	return fmt.Sprintf("zylo-%d", time.Now().UnixNano())
}
