package hclconfig

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/flytam/filenamify"
	getter "github.com/hashicorp/go-getter"
)

type Getter interface {
	// Get fetches the source files from src and downloads them to the
	// given folder. If the files already exist at the given location
	// Get does nothing unless ignoreCache is true when source will be
	// downloaded regarless of cache.
	//
	// Get returns a string with the full path of the downloaded source
	// this contains any url characters in src correctly encoded for
	// a filepath.
	Get(src, destFolder string, ignoreCache bool) (string, error)
}

type GoGetter struct {
	get func(src, dest, working string) error
}

func NewGoGetter() Getter {
	return &GoGetter{
		get: func(src, dest, working string) error {
			c := &getter.Client{
				Ctx:     context.Background(),
				Src:     src,
				Dst:     dest,
				Pwd:     working,
				Mode:    getter.ClientModeAny,
				Options: []getter.ClientOption{},
			}

			err := c.Get()
			if err != nil {
				return fmt.Errorf("unable to fetch files from %s: %s", src, err)
			}

			return nil
		},
	}
}

func (g *GoGetter) Get(src, dest string, ignoreCache bool) (string, error) {
	// check to see if a folder exists at the destination and exit if exists

	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// ensure the output folder is correctly encoded
	output, err := filenamify.Filenamify(src, filenamify.Options{
		Replacement: "_",
	})

	downloadPath := path.Join(dest, output)

	// check to see if the destination exists
	_, err = os.Stat(downloadPath)
	if err == nil && !ignoreCache {
		return downloadPath, nil
	}

	err = g.get(src, downloadPath, pwd)

	return downloadPath, err
}
