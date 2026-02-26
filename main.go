package main

import (
	"bufio"
	"em_webshare/core"
	"fmt"
	"os"
)

func handleCLICommands() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("Enter command ('upload <path>', 'ls <path>', 'cd <path>', 'pwd', 'up_dir <path>', 'exit'): ")

		if !scanner.Scan() {
			break
		}

		command := scanner.Text()

		if !core.HandleCommand(command, os.Stdout) {
			return
		}
	}
}

func main() {
	fmt.Println("EM WebShare : Simple Web Based file sharing app")
	fmt.Print("contribute : https://github.com/SkillfulElectro/em_webshare.git\n\n")

	core.Init("")

	port := core.FindAvailablePort()
	if port == -1 {
		fmt.Println("No available ports in the range 8000-60000.")
		return
	}

	go core.StartServer(port)

	handleCLICommands()
}
