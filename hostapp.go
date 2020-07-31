package hostapp

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/docker/docker/daemon/graphdriver/aufs"
	_ "github.com/docker/docker/daemon/graphdriver/overlay2"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/idtools"
)

type HostConfig struct {
	Labels map[string]string `json:"Labels"`
}

type Config struct {
	ID         string `json:"ID"`
	Image      string `json:"Image"`
	HostConfig `json:"Config"`
	Name       string `json:"Name"`
	Driver     string `json:"Driver"`
}

type Container struct {
	Config
	MountPath string
}

var Containers []Container

var debug bool = false
var verbose bool = false

// Testing stub substitution
var rwLayerMount = layer.RWLayer.Mount  // As root, do not mount layer
var containerMount = (*Container).mount // As user, do not call mount

func (container *Container) mount(layer_root string) string {
	ls, err := layer.NewStoreFromOptions(layer.StoreOptions{
		Root:                      layer_root,
		MetadataStorePathTemplate: filepath.Join(layer_root, "image", "%s", "layerdb"),
		IDMapping:                 &idtools.IdentityMapping{},
		GraphDriver:               container.Config.Driver,
		OS:                        runtime.GOOS,
	})
	if err != nil {
		log.Fatal("error loading layer store:", err)
	}

	rwlayer, err := ls.GetRWLayer(container.Config.ID)
	if err != nil {
		log.Fatal("error getting container layer:", err)
	}

	newRoot, err := rwLayerMount(rwlayer, "")
	if err != nil {
		log.Fatal("error mounting container fs:", err)
	}
	container.MountPath = newRoot.Path()
	/*
		if err := unix.Mount("", newRootPath, "", unix.MS_REMOUNT, ""); err != nil {
			log.Fatal("error remounting container as read/write:", err)
		}
	*/

	return container.MountPath
}

func (container *Container) initialize(homePath string) error {
	configPath := filepath.Join(homePath, string(os.PathSeparator), "config.v2.json")
	f, err := os.Open(configPath)
	if err != nil {
		fmt.Printf("%s\n", err)
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&container.Config); err != nil {
		log.Println("Error decoding config file:", err)
		return err
	}
	if verbose || debug {
		log.Println("Initialized container:", container.Config.Name)
	}
	if debug {
		log.Printf("%+v\n", container.Config)
	}
	return nil
}

func (container *Container) mountOverlay(mountRoot string, targetLabel string) (string, error) {
	if debug == true {
		log.Printf("Searching for label %s\n", targetLabel)
	}
	for label, _ := range container.Labels {
		if label == targetLabel {
			if verbose {
				log.Printf("Mounting %s in %s\n", label, mountRoot)
			}
			newRootPath := containerMount(container, mountRoot)
			return newRootPath, nil
		}
	}
	return "", fmt.Errorf("Label %s not found\n", targetLabel)
}

// Mount a container union filesystem by label
func MountContainersByLabel(rootdir string, label string) ([]Container, error) {
	containersDir := filepath.Join(rootdir, "containers")
	dirs, err := ioutil.ReadDir(containersDir)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	Containers := make([]Container, 0)
	for i, dir := range dirs {
		if dir.IsDir() {
			homePath := filepath.Join(containersDir, string(os.PathSeparator), dir.Name())
			Containers = append(Containers, Container{})
			if Containers[i].initialize(homePath) != nil {
				log.Println("Error initializing container")
				return nil, err
			}
			if debug {
				log.Printf("Trying to mount label %s from %s\n", label, rootdir)
			}
			Containers[i].mountOverlay(rootdir, label)
		}
	}
	return Containers, nil
}
