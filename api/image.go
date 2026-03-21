package api

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"zylo/global"
)

const (
	registryPrefix = "ghcr.io/ivankr8/"
	defaultTag     = ":1.0"
)

func PullImage(imageName string) (err error) {
	fullRef := registryPrefix + imageName + defaultTag

	ref, err := name.ParseReference(fullRef)
	if err != nil {
		return err
	}

	finalDir := filepath.Join(global.ImgsPth, Sanitize(imageName))
	tmpDir := finalDir + ".tmp"
	lockFile := finalDir + ".lock"

	if _, err := os.Stat(finalDir); err == nil {
		return nil
	}

	if _, err := os.Stat(lockFile); err == nil {
		return fmt.Errorf("image is currently downloading")
	}

	if err = os.WriteFile(lockFile, []byte("1"), 0644); err != nil {
		return err
	}

	defer func() {
		os.Remove(lockFile)
		if err != nil {
			os.RemoveAll(tmpDir)
		}
	}()

	if err = os.RemoveAll(tmpDir); err != nil {
		return err
	}

	if err = os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}

	img, err := remote.Image(
		ref,
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	)
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return err
	}

	for _, layer := range layers {
		if err = extractLayer(layer, tmpDir); err != nil {
			return err
		}
	}

	if err = os.Rename(tmpDir, finalDir); err != nil {
		return err
	}

	return nil
}

func extractLayer(layer v1.Layer, dest string) error {
	rc, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	defer rc.Close()

	tr := tar.NewReader(rc)

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		name := h.Name
		if strings.Contains(name, "..") {
			continue
		}

		target := filepath.Join(dest, name)
		base := filepath.Base(name)

		if strings.HasPrefix(base, ".wh.") {
			remove := filepath.Join(filepath.Dir(target), strings.TrimPrefix(base, ".wh."))
			os.RemoveAll(remove)
			continue
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(h.Mode)); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			f.Close()
			if err != nil {
				return err
			}

		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Symlink(h.Linkname, target)

		case tar.TypeLink:
			linkTarget := filepath.Join(dest, h.Linkname)
			os.Link(linkTarget, target)
		}
	}

	return nil
}

func Sanitize(name string) string {
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return name
}

func ImageExists(imageName string) bool {
	imagePath := filepath.Join(global.ImgsPth, Sanitize(imageName))
	_, err := os.Stat(imagePath)
	return err == nil
}

func ImageRemove(imageName string) error {
	imageDir := filepath.Join(global.ImgsPth, Sanitize(imageName))
	return os.RemoveAll(imageDir)
}
