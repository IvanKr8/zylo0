package network

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"zylo/internal/global"
	"zylo/internal/system"
)

type MockSystem struct {
	mock.Mock
}

func TestNewBridge(t *testing.T) {
	tmpDir := t.TempDir()
	global.NetPth = tmpDir

	var commands []string
	system.Command = func(name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil
	}

	checkIface = func(name string) (bool, error) {
		return false, nil
	}

	err := NewBridge("testbridge")
	assert.NoError(t, err)

	path := filepath.Join(tmpDir, "testbridge.json")
	_, err = os.Stat(path)
	assert.NoError(t, err)

	data, err := os.ReadFile(path)
	assert.NoError(t, err)

	var net network
	json.Unmarshal(data, &net)
	assert.Equal(t, "testbridge", net.Name)
	assert.NotEmpty(t, net.IP)
	assert.NotEmpty(t, net.Subnet)

	assert.Len(t, commands, 3)
	assert.Contains(t, commands[0], "ip")
	assert.Contains(t, commands[0], "link")
	assert.Contains(t, commands[0], "add")

	assert.Contains(t, commands[1], "addr")
	assert.Contains(t, commands[1], "add")

	assert.Contains(t, commands[2], "link")
	assert.Contains(t, commands[2], "set")

}

func (m *MockSystem) Command(name string, args ...string) error {
	argsList := append([]interface{}{name}, convertArgsToInterfaces(args)...)
	return m.Called(argsList...).Error(0)
}

func convertArgsToInterfaces(args []string) []interface{} {
	var result []interface{}
	for _, arg := range args {
		result = append(result, arg)
	}
	return result
}

func TestCreateVethPair(t *testing.T) {
	mockSys := new(MockSystem)

	mockSys.On("Command", "ip", "link", "add", "test-host", "type", "veth", "peer", "name", "test-container").Return(nil)
	mockSys.On("Command", "ip", "link", "set", "test-host", "master", "test-container").Return(nil)
	mockSys.On("Command", "ip", "link", "set", "test-host", "up").Return(nil)
	mockSys.On("Command", "ip", "link", "set", "test-container", "up").Return(nil)

	originalSystemCommand := system.Command
	defer func() { system.Command = originalSystemCommand }()
	system.Command = mockSys.Command

	err := CreateVethPair("test", "test")

	assert.NoError(t, err)

	mockSys.AssertExpectations(t)
}
