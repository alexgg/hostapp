package hostapp

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/containerfs"
)

var rootdir = flag.String("rootdir", "/path/to/rootdir", "Path to root directory")

var mountedContainers int

func stubMount(layer.RWLayer, string) (containerfs.ContainerFS, error) {
	mountedContainers++
	if debug {
		log.Printf("Stub rwLayer mount: %d\n", mountedContainers)
	}
	return nil, nil
}

func stubContainerMount(*Container, string) string {
	mountedContainers++
	if debug {
		log.Printf("Stub container mount %d\n", mountedContainers)
	}
	return ""
}

func TestMountContainersByLabel(t *testing.T) {
	var tests = []struct {
		rootdir       string
		label         string
		expectFailure bool
	}{
		{"/does/not/exist", "None", true},
		{"/link/to/rootdir", "unique-label", false},
		{"/path/to/file", "None", true},
		{"/path/to/rootdir", "unique-label", false},
		{"/path/to/rootdir", "nonsense", false},
		{"/path/to/rootdir", "repeated-label", false},
	}

	savedRwLayerMount := rwLayerMount
	rwLayerMount = stubMount
	defer func() { rwLayerMount = savedRwLayerMount }()

	savedContainerMount := containerMount
	containerMount = stubContainerMount
	defer func() { containerMount = savedContainerMount }()

	if *rootdir == "" {
		log.Fatal("This test requires a --rootdir flag")
	}

	linkRootDir := "/tmp/testlink"
	os.Symlink(*rootdir, linkRootDir)
	defer os.Remove(linkRootDir)

	fileRootDir, err := ioutil.TempFile("", "testHostAppFile")
	if err != nil {
		log.Fatal("Unable to create temporary file")
	}

	tests[1].rootdir = linkRootDir
	tests[2].rootdir = fileRootDir.Name()
	tests[3].rootdir = *rootdir
	tests[4].rootdir = *rootdir
	tests[5].rootdir = *rootdir

	for _, test := range tests {
		mountedContainers = 0
		containers, err := MountContainersByLabel(test.rootdir, test.label)
		if test.expectFailure == true && err == nil {
			t.Errorf("Test with rootdir %s and label %s should have failed", test.rootdir, test.label)
		} else if test.expectFailure == false && err != nil {
			t.Errorf("Test with rootdir %s and label %s should have passed", test.rootdir, test.label)
		}
		if test.label == "unique-label" && mountedContainers > 1 {
			t.Errorf("Test with rootdir %s and label %s should return just one container, not %d", test.rootdir, test.label, mountedContainers)
		}
		if test.label == "repeated-label" && mountedContainers != len(containers)-1 {
			t.Errorf("Test with rootdir %s and label %s should return %d containers, not %d", test.rootdir, test.label, len(containers)-1, mountedContainers)
		}
	}
}

func BenchmarkMountSingleContainer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MountContainersByLabel("/path/to/rootdir", "unique-label")
	}
}

func BenchmarkMountMultipleContainer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MountContainersByLabel("/path/to/rootdir", "repeated-label")
	}
}

func ExampleMountContainersByLabel() {
	MountContainersByLabel("/path/to/rootdir", "unique-label")
}
