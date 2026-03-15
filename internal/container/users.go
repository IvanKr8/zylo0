package kernel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type User struct {
	Name     string
	UID      int
	GID      int
	Home     string
	Shell    string
	Password string
}

type Group struct {
	Name  string
	GID   int
	Users []string
}

type UserManager struct {
	rootfs string
	users  []User
	groups []Group
}

func NewUserManager(rootfs string) *UserManager {
	return &UserManager{
		rootfs: rootfs,
		users:  []User{},
		groups: []Group{},
	}
}

func (u *UserManager) AddUser(user User) {
	u.users = append(u.users, user)
}

func (u *UserManager) AddGroup(group Group) {
	u.groups = append(u.groups, group)
}

func (u *UserManager) WriteFiles() error {
	// Создаем etc директорию если нет
	etcDir := filepath.Join(u.rootfs, "etc")
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		return fmt.Errorf("failed to create etc dir: %v", err)
	}

	// /etc/passwd
	passwdPath := filepath.Join(etcDir, "passwd")
	passwdFile, err := os.OpenFile(passwdPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open passwd: %v", err)
	}
	defer passwdFile.Close()

	for _, user := range u.users {
		passwd := fmt.Sprintf("%s:%s:%d:%d:%s:%s:%s\n",
			user.Name,
			user.Password,
			user.UID,
			user.GID,
			user.Name,
			user.Home,
			user.Shell,
		)
		if _, err := passwdFile.WriteString(passwd); err != nil {
			return err
		}
	}

	// /etc/group
	groupPath := filepath.Join(etcDir, "group")
	groupFile, err := os.OpenFile(groupPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open group: %v", err)
	}
	defer groupFile.Close()

	for _, group := range u.groups {
		groupLine := fmt.Sprintf("%s:x:%d:%s\n",
			group.Name,
			group.GID,
			strings.Join(group.Users, ","),
		)
		if _, err := groupFile.WriteString(groupLine); err != nil {
			return err
		}
	}

	return nil
}

func (u *UserManager) AddUsersFromMetadata(meta *ImageMetadata) {
	if meta == nil {
		return
	}

	for _, group := range meta.Groups {
		u.AddGroup(Group{
			Name:  group.Name,
			GID:   group.GID,
			Users: group.Users,
		})
	}

	if meta.DefaultUser != nil {
		u.AddUser(User{
			Name:     meta.DefaultUser.Name,
			UID:      meta.DefaultUser.UID,
			GID:      meta.DefaultUser.GID,
			Home:     meta.DefaultUser.Home,
			Shell:    meta.DefaultUser.Shell,
			Password: meta.DefaultUser.Password,
		})
	}
}

func (u *UserManager) AddDefaultRoot() {
	u.AddGroup(Group{
		Name:  "root",
		GID:   0,
		Users: []string{"root"},
	})

	u.AddUser(User{
		Name:     "root",
		UID:      0,
		GID:      0,
		Home:     "/root",
		Shell:    "/bin/sh",
		Password: "x",
	})
}
