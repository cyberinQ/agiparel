package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

func speak(text string) {
	fmt.Printf("\n🎙️  [Agie]: %s\n", text)
	if runtime.GOOS == "darwin" {
		_ = exec.Command("say", text).Run()
	}
}

func main() {
	fmt.Println("==================================================================")
	fmt.Println("  AGIE: Agiparel Onboarding Gateway")
	fmt.Println("==================================================================")

	speak("Welcome to Agiparel. Your system is ready to configure. Please open your browser to http://localhost:8080/onboarding to complete your interactive factory setup.")

	fmt.Println("\n------------------------------------------------------------------")
	fmt.Println(" 👉 Open your browser and navigate to:")
	fmt.Println("    http://localhost:8080/onboarding")
	fmt.Println("------------------------------------------------------------------\n")
}