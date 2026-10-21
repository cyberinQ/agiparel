package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
)

func speak(text string) {
	fmt.Printf("\n🎙️  [Agie]: %s\n", text)
	if runtime.GOOS == "darwin" {
		_ = exec.Command("say", text).Run()
	}
}

func main() {
	fmt.Println("==================================================================")
	fmt.Println("  AGIE: Pre-Boarding Spreadsheet Ingestion & Factory Configurator")
	fmt.Println("==================================================================")

	templatePath := "templates/factory_onboarding_template.csv"
	_ = os.MkdirAll("templates", 0755)
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		_ = os.WriteFile(templatePath, []byte(onboarding.GenerateTemplateCSV()), 0644)
	}

	speak("Welcome to Agiparel OS. Instead of using hardcoded demo data, we configure your entire factory from your Pre-Boarding Workbook. A template with examples in every section has been prepared at templates/factory_onboarding_template.csv.")

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n[Enter path to your filled Pre-Boarding CSV Workbook (press Enter for '%s')]: ", templatePath)
	inputPath, _ := reader.ReadString('\n')
	inputPath = strings.TrimSpace(inputPath)
	if inputPath == "" {
		inputPath = templatePath
	}

	csvBytes, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Fatal: unable to read workbook at %s: %v\n", inputPath, err)
		return
	}

	wb, err := onboarding.ParseWorkbookCSV(bytes.NewReader(csvBytes))
	if err != nil {
		fmt.Printf("Workbook validation error: %v\n", err)
		return
	}

	fmt.Printf("\n--- PRE-BOARDING WORKBOOK SUMMARY ---")
	fmt.Printf("\nFactory / Unit : %s (%s) [Tenant: %s]", wb.FactoryName, wb.UnitCode, wb.TenantID)
	fmt.Printf("\nLines          : %d lines", len(wb.Lines))
	fmt.Printf("\nMachines       : %d machines", len(wb.Machines))
	fmt.Printf("\nWorkers        : %d workers", len(wb.Workers))
	fmt.Printf("\nStyles & Parts : %d styles, %d parts, %d DAG operations", len(wb.Styles), len(wb.StyleParts), len(wb.OperationsDAG))
	fmt.Printf("\nStore Rolls    : %d rolls", len(wb.StoreRolls))
	fmt.Printf("\nWork Orders    : %d active POs\n", len(wb.WorkOrders))

	speak("Please review the parsed summary from your spreadsheet. Type CONFIRM to provision the database and edge engines.")
	fmt.Print("\n[Type CONFIRM to apply]: ")
	confirmInput, _ := reader.ReadString('\n')
	if strings.TrimSpace(confirmInput) != "CONFIRM" {
		fmt.Println("Aborted by user.")
		return
	}

	resp, postErr := http.Post("http://localhost:8080/api/v1/onboarding/upload", "text/csv", bytes.NewReader(csvBytes))
	if postErr != nil {
		fmt.Printf("Warning: Failed to reach edge daemon: %v\n", postErr)
		return
	}
	defer resp.Body.Close()
	bodyOut, _ := io.ReadAll(resp.Body)

	var summary map[string]interface{}
	_ = json.Unmarshal(bodyOut, &summary)
	pretty, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Printf("\nServer Response:\n%s\n", string(pretty))

	speak("Factory onboarding complete! All lines, machines, employees, skills, styles, parts, and rolls are now live.")
}