package entry

import (
	"errors"
	"strings"
)

// SplitCommand splits a command into arguments: spaces and tabs separate, single and
// double quotes group. Nothing else is special, because a command never reaches a shell.
func SplitCommand(command string) ([]string, error) {
	var (
		args    []string
		current strings.Builder
		inArg   bool
		quote   rune
	)
	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inArg = true
		case r == ' ' || r == '\t':
			if inArg {
				args = append(args, current.String())
				current.Reset()
				inArg = false
			}
		default:
			current.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, errors.New("a quote is not closed")
	}
	if inArg {
		args = append(args, current.String())
	}
	if len(args) == 0 {
		return nil, errors.New("the command is empty")
	}
	if args[0] == "" {
		return nil, errors.New("the command has an empty program name")
	}
	return args, nil
}
