package common

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
)

//var CommandStr string
//var TargetStr string

const (

	// NotDefined is a variable that holds the string "Not_Defined"
	NotDefined string = "Not_Defined"
)

// SysCall executes user-passed command via system call.
func SysCall(cmdStr string) {
	//cmdStr := "docker-compose -f " + common.DockerFilePath + " up"
	cmd := exec.Command("/bin/sh", "-c", cmdStr)

	cmdReader, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	err := cmd.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(cmdReader)
	for scanner.Scan() {
		fmt.Printf("%s\n", scanner.Text())
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Println(err)
		//os.Exit(1)
	}

}

// SysCallWithError executes user-passed command via system call and returns error.
func SysCallWithError(cmdStr string) error {
	cmd := exec.Command("/bin/sh", "-c", cmdStr)

	cmdReader, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	err := cmd.Start()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(cmdReader)
	for scanner.Scan() {
		fmt.Printf("%s\n", scanner.Text())
	}

	err = cmd.Wait()
	return err
}
