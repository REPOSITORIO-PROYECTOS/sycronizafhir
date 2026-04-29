package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	if runtime.GOOS != "windows" {
		fmt.Println("Este instalador solo funciona en Windows.")
		os.Exit(1)
	}

	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("No se pudo obtener ruta del ejecutable: %v\n", err)
		os.Exit(1)
	}

	baseDir := filepath.Dir(exePath)
	scriptPath := filepath.Join(baseDir, "instalar-sycronizafhir.ps1")
	if _, err = os.Stat(scriptPath); err != nil {
		fmt.Printf("No se encontro instalar-sycronizafhir.ps1 junto al instalador.\n")
		os.Exit(1)
	}

	args := []string{
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-Command",
		fmt.Sprintf("Start-Process powershell -Verb RunAs -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File \"%s\"'", scriptPath),
	}
	cmd := exec.Command("powershell.exe", args...)
	if err = cmd.Run(); err != nil {
		fmt.Printf("No se pudo iniciar el instalador elevado: %v\n", err)
		os.Exit(1)
	}
}

