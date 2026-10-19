// SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC. All rights reserved.
// SPDX-FileCopyrightText: © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Ported from ochami's internal/log/color.go (MIT) to provide consistent
// ANSI colorization of log messages across OpenCHAMI tools.
package log

import "fmt"

const (
	colorBlack = iota + 30
	colorRed
	colorGreen
	colorYellow
	colorBlue
	colorMagenta
	colorCyan
	colorWhite

	colorBold     = 1
	colorDarkGray = 90
)

// colorize returns the string s wrapped in ANSI code c, unless disabled is
// true or c is 0. Unlike ochami's version, the NO_COLOR environment variable
// is not checked here; it is handled centrally by noColor() so that a single
// decision controls every rendered part.
func colorize(s interface{}, c int, disabled bool) string {
	if disabled || c == 0 {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}
