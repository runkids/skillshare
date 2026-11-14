package utils

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var coreDecimal = regexp.MustCompile(`^[-+]?[0-9]+$`)
var coreNumber = regexp.MustCompile(`^(0o[0-7]+|0x[0-9a-fA-F]+|[-+]?(\.[0-9]+|[0-9]+(\.[0-9]*)?)([eE][-+]?[0-9]+)?|[-+]?\.(inf|Inf|INF)|\.(nan|NaN|NAN))$`)

// yaml.v3 resolves timestamps and legacy numeric forms implicitly. Skills clients
// parse the served YAML with the YAML 1.2 Core schema: dates stay strings and
// decimal integers with leading zeroes are base ten. Normalize only this strict
// JSON view; leave source bytes and the lenient readers untouched.
func normalizeCoreScalars(n *yaml.Node) {
	if n.Kind == yaml.ScalarNode {
		explicit := n.Style&yaml.TaggedStyle != 0
		if !explicit && (n.Tag == "!!timestamp" || ((n.Tag == "!!int" || n.Tag == "!!float") && !coreNumber.MatchString(n.Value))) {
			n.Tag = "!!str"
		}
		if n.Tag == "!!int" && coreDecimal.MatchString(n.Value) {
			sign, digits := "", n.Value
			if digits[0] == '+' || digits[0] == '-' {
				sign, digits = digits[:1], digits[1:]
			}
			digits = strings.TrimLeft(digits, "0")
			if digits == "" {
				digits = "0"
			}
			n.Value = sign + digits
		}
	}
	// Aliases point at nodes already in Content; following them can recurse forever.
	for _, child := range n.Content {
		normalizeCoreScalars(child)
	}
}
