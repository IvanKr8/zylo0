package container

import (
	"fmt"
	"os"
	"zylo/internal/system"
)

func prepareDirs(containerCfg *container) error {
	workdir := containerCfg.workdir

	if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create workdir: %v", err)
	}

	err := os.Mkdir(containerCfg.rootfs+"/work", os.ModePerm)
	if err != nil {
		return err
	}
	err = os.Mkdir(containerCfg.rootfs+"/merged", os.ModePerm)
	if err != nil {
		return err
	}
	err = os.Mkdir(containerCfg.rootfs+"/source/tmp", os.ModePerm)
	if err != nil {
		return err
	}
	err = os.Mkdir(containerCfg.rootfs+"/source/dev", os.ModePerm)
	if err != nil {
		return err
	}

	//_, err = os.Create(containerCfg.rootfs + "/source/log/_init_log.json")
	//if err != nil {
	//	return err
	//}
	//_, err = os.Create(containerCfg.rootfs + "/source/log/" + containerCfg.hash + "_log.json")
	//if err != nil {
	//	return err
	//}

	return nil
}

func prepareWorkDir(containerCfg *container) error {
	srcDir := containerCfg.copyDir
	destDir := containerCfg.workdir

	fi, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("failed to stat source directory %s: %v", srcDir, err)
	}

	if !fi.IsDir() {
		return fmt.Errorf("source path %s is not a directory", srcDir)
	}

	if err = os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", destDir, err)
	}

	if err = system.CpDir(srcDir, destDir); err != nil {
		return fmt.Errorf("failed to copy files from %s to %s: %v", srcDir, destDir, err)
	}

	return nil
}
