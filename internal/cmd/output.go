package cmd

import "fmt"

type outputMode string

const (
	outputModeText outputMode = "text"
	outputModeJSON outputMode = "json"
	outputModeTUI  outputMode = "tui"
)

func outputModeOf(value string) (outputMode, error) {
	switch mode := outputMode(value); mode {
	case outputModeText, outputModeJSON, outputModeTUI:
		return mode, nil
	default:
		return "", fmt.Errorf("unknown output mode %q: want text, json, or tui", value)
	}
}
