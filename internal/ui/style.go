package ui

import "charm.land/lipgloss/v2"

var (
	cAccent = lipgloss.Color("#FF79C6")
	cMint   = lipgloss.Color("#50FA7B")
	cSun    = lipgloss.Color("#F1FA8C")
	cRose   = lipgloss.Color("#FF5555")
	cViolet = lipgloss.Color("#BD93F9")
	cSky    = lipgloss.Color("#8BE9FD")
	cDim    = lipgloss.Color("#7F849C")

	sDim     = lipgloss.NewStyle().Foreground(cDim)
	sBold    = lipgloss.NewStyle().Bold(true)
	sAccent  = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	sGroup   = lipgloss.NewStyle().Foreground(cSky).Bold(true)
	sVersion = lipgloss.NewStyle().Foreground(cViolet)
	sWarn    = lipgloss.NewStyle().Foreground(cSun)
	sErr     = lipgloss.NewStyle().Foreground(cRose)
	sOK      = lipgloss.NewStyle().Foreground(cMint).Bold(true)
)
