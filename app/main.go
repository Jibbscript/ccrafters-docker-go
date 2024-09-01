package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const tmpDir = "/tmp/fockerfs"

func getDockerToken(image string) string {
	// Get token from docker hub
	type AuthResponse struct {
		Token string `json:"token"`
	}
	// imageName := strings.Split(image, ":")
	var authResponse AuthResponse
	authUrl := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%v:pull"
	if res, err := http.Get(fmt.Sprintf(authUrl, image)); err == nil {
		if res.StatusCode == 200 {
			json.NewDecoder(res.Body).Decode(&authResponse)
		}
	}
	return authResponse.Token
}

type BlobSum struct {
	BlobSum string `json:"blobSum"`
}
type Manifest struct {
	Layers []BlobSum `json:"fsLayers"`
}

func getImageManifest(image string, token string) []BlobSum {
	// Get image manifest
	// imageName := strings.Split(image, ":")
	var manifest Manifest
	manifestUrl := "https://registry.hub.docker.com/v2/library/%v/manifests/latest"
	if res, err := http.NewRequest("GET", fmt.Sprintf(manifestUrl, image), nil); err == nil {
		res.Header.Set("Authorization", "Bearer "+token)
		client := &http.Client{}
		if resp, err := client.Do(res); err == nil {
			if resp.StatusCode == 200 {
				json.NewDecoder(resp.Body).Decode(&manifest)
			}
		}
	}
	return manifest.Layers
}

func getLayerBlob(image, blobSum, token string) error {
	layerUrl := fmt.Sprintf("https://registry-1.docker.io/v2/library/%v/blobs/%v", image, blobSum)
	res, err := http.NewRequest("GET", layerUrl, nil)
	if err != nil {
		fmt.Println("Error: ", err)
	}
	res.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(res)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	fileName := filepath.Join("/tmp/", blobSum)
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	_, readErr := file.ReadFrom(resp.Body)
	if readErr != nil {
		return readErr
	}
	if err := exec.Command("tar", "xf", fileName, "-C", tmpDir).Run(); err != nil {
		return errors.New("Error extracting layer: " + err.Error())
	}
	return nil
}

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	execDir := filepath.Join(tmpDir, filepath.Dir(command))
	os.MkdirAll(tmpDir, 0744)
	os.MkdirAll(execDir, 0744)
	exec.Command("cp %s %s", command, tmpDir).Run()
	token := getDockerToken(os.Args[2])
	layers := getImageManifest(os.Args[2], token)
	for _, layer := range layers {
		if err := getLayerBlob(os.Args[2], layer.BlobSum, token); err != nil {
			fmt.Println("Error: ", err)
		}
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:     tmpDir,
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID,
	}

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		} else {
			fmt.Printf("Err: %v", err)
			os.Exit(1)
		}
	}

}
