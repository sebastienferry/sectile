package agent

import (
	"fmt"
	"strings"
	"unicode"
)

// shellCommandWords only reads the command Sectile built; it never evaluates
// substitutions or executes a shell. Quoted prompts stay one argument.
func shellCommandWords(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote rune
	escaped, started := false, false
	for _, ch := range line {
		if escaped {
			word.WriteRune(ch)
			escaped = false
			started = true
			continue
		}
		if ch == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			} else {
				word.WriteRune(ch)
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			started = true
			continue
		}
		if unicode.IsSpace(ch) {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(ch)
		started = true
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("incomplete shell argument")
	}
	if started {
		words = append(words, word.String())
	}
	return words, nil
}
