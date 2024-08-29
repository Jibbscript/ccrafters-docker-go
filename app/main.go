package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func createChrootDir() string {
	dir, err := os.MkdirTemp("", "fockerfs")
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("mkdir -p %s/usr/local/bin && cp /usr/local/bin/docker-explorer %s/usr/local/bin/docker-explorer", dir, dir))
	if err := cmd.Run(); err != nil {
		panic(err)
	}
	return dir
}

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	switch fockerCmd := os.Args[1]; fockerCmd {
	case "run":
		dir := createChrootDir()
		defer os.RemoveAll(dir)

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Chroot: dir,
		}

		err := cmd.Run()
		if err != nil {
			var exitCode int
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}

			os.Exit(exitCode)
		}
	default:
		panic(fmt.Sprintf("Invalid command: %s", command))
	}
}
