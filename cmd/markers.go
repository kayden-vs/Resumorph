package cmd

import "regexp"

var markerRe = regexp.MustCompile(`%%([A-Z0-9_]+)%%`)

const okMark = "\u2713"
