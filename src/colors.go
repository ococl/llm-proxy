package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	colorError   = lipgloss.Color("#FF5555")
	colorWarn    = lipgloss.Color("#FFB86C")
	colorInfo    = lipgloss.Color("#50FA7B")
	colorDebug   = lipgloss.Color("#BD93F9")
	colorReqID   = lipgloss.Color("#8BE9FD")
	colorTime    = lipgloss.Color("#6272A4")
	colorModel   = lipgloss.Color("#FF79C6")
	colorBackend = lipgloss.Color("#F1FA8C")
	colorSuccess = lipgloss.Color("#50FA7B")
	colorLatency = lipgloss.Color("#FFB86C")

	errorStyle   = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(colorInfo)
	debugStyle   = lipgloss.NewStyle().Foreground(colorDebug)
	reqIDStyle   = lipgloss.NewStyle().Foreground(colorReqID).Bold(true)
	timeStyle    = lipgloss.NewStyle().Foreground(colorTime)
	modelStyle   = lipgloss.NewStyle().Foreground(colorModel)
	backendStyle = lipgloss.NewStyle().Foreground(colorBackend)
	latencyStyle = lipgloss.NewStyle().Foreground(colorLatency)

	reqIDPattern   = regexp.MustCompile(`\[req_[a-fA-F0-9]+\]`)
	modelPattern   = regexp.MustCompile(`模型=([^\s]+)`)
	backendPattern = regexp.MustCompile(`后端=([^\s]+)`)
	latencyPattern = regexp.MustCompile(`耗时=(\d+ms)`)
)

func shouldUseColor() bool {
	if loggingConfig != nil && loggingConfig.Colorize != nil && !*loggingConfig.Colorize {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

func colorLevel(level string) string {
	switch strings.ToUpper(level) {
	case "ERROR":
		return errorStyle.Render("ERROR")
	case "WARN":
		return warnStyle.Render("WARN ")
	case "INFO":
		return infoStyle.Render("INFO ")
	case "DEBUG":
		return debugStyle.Render("DEBUG")
	default:
		return level
	}
}

func colorTimeStr(t string) string {
	return timeStyle.Render(t)
}

func highlightRequestID(msg string) string {
	result := reqIDPattern.ReplaceAllStringFunc(msg, func(match string) string {
		return reqIDStyle.Render(match)
	})
	result = modelPattern.ReplaceAllStringFunc(result, func(match string) string {
		parts := modelPattern.FindStringSubmatch(match)
		if len(parts) > 1 {
			return "模型=" + modelStyle.Render(parts[1])
		}
		return match
	})
	result = backendPattern.ReplaceAllStringFunc(result, func(match string) string {
		parts := backendPattern.FindStringSubmatch(match)
		if len(parts) > 1 {
			return "后端=" + backendStyle.Render(parts[1])
		}
		return match
	})
	result = latencyPattern.ReplaceAllStringFunc(result, func(match string) string {
		parts := latencyPattern.FindStringSubmatch(match)
		if len(parts) > 1 {
			return "耗时=" + latencyStyle.Render(parts[1])
		}
		return match
	})
	return result
}

func printBanner(version, listen string, backends, models int) {
	if testMode || !shouldUseColor() {
		return
	}

	banner := `
╦  ╦  ╔╦╗  ╔═╗┬─┐┌─┐─┐ ┬┬ ┬
║  ║  ║║║  ╠═╝├┬┘│ │┌┴┬┘└┬┘
╩═╝╩═╝╩ ╩  ╩  ┴└─└─┘┴ └─ ┴ `

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9")).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))

	fmt.Println(titleStyle.Render(banner))
	fmt.Println()
	fmt.Println(labelStyle.Render("  Version:  ") + valueStyle.Render(version))
	fmt.Println(labelStyle.Render("  Listen:   ") + valueStyle.Render(listen))
	fmt.Println(labelStyle.Render("  Backends: ") + valueStyle.Render(fmt.Sprintf("%d loaded", backends)))
	fmt.Println(labelStyle.Render("  Models:   ") + valueStyle.Render(fmt.Sprintf("%d aliases", models)))
	fmt.Println()
}
