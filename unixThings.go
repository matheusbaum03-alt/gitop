package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func prettyPrintInfo() {
	fmt.Println("gitop: the source based omni package manager")
	fmt.Println("use: gitop <task> <repo>")
	fmt.Println("tasks: build, install, unistall, add and update")
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("error while getting home dir returning $HOME with path")
		return filepath.Join(os.Getenv("HOME"), ".config", "gitop", "config.json")
	}
	return filepath.Join(home, ".config", "gitop", "config.json")
}

func getCachePath(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("error while getting home dir returning $HOME with path")
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "gitop", name)
	}
	return filepath.Join(home, ".local", "share", "gitop", name)
}
