package network

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"zylo/internal/global"
)

const (
	MINIPVALUE = 0
	MAXIPVALUE = 255
)

type network struct {
	Name       string   `json:"name"`
	IP         string   `json:"ip"`
	Subnet     string   `json:"subnet"`
	Containers []string `json:"containers"`
}

func NewBridge(name string) error {
	ip, subnet, err := generateUniqueIP()
	if err != nil {
		return err
	}

	exists, err := checkIface(name)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	if err = createBridge(name); err != nil {
		return err
	}

	if err = assignIPtoBridge(name, ip); err != nil {
		return err
	}

	if err = upIface(name); err != nil {
		return fmt.Errorf("failed to bring up network %s: %v", name, err)
	}

	net := network{
		Name:   name,
		IP:     ip,
		Subnet: subnet,
	}
	data, _ := json.MarshalIndent(net, "", "  ")
	if err := os.WriteFile(filepath.Join(global.NetPth, name+".json"), data, 0644); err != nil {
		return fmt.Errorf("failed to write network config: %v", err)
	}

	return nil
}

func CreateVethPair(pair1, pair2 string) error {
	exists1, err := checkIface(pair1)
	if err != nil {
		return err
	}

	exists2, err := checkIface(pair2)
	if err != nil {
		return err
	}

	if exists1 || exists2 {
		return nil
	}

	if err := createVeth(pair1, pair2); err != nil {
		return err
	}

	if err := tiePair(pair1, pair2); err != nil {
		return err
	}

	if err := upIface(pair1); err != nil {
		return fmt.Errorf("failed to bring up pair %s: %v", pair1, err)
	}
	if err := upIface(pair2); err != nil {
		return fmt.Errorf("failed to bring up pair %s: %v", pair2, err)
	}

	return nil
}

//func TiePair(pair) {
//
//}

func CreateVeth(pair1, pair2 string) error {
	return createVeth(pair1, pair2)
}

func UpIface(pair string) error {
	return upIface(pair)
}
