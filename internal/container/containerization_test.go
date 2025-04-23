package container

import "testing"

func TestSetNS(t *testing.T) {
	containerCfg := container{
		rootfs: "/tmp/zylo/test1",
		image:  "go",
	}

	err := setNS(&containerCfg)
	if err != nil {
		t.Error(err)
		return
	}

	t.Log("All is good")
}
