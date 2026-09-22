package cli

import (
	"bufio"
	"io"
	"strings"
)

func confirm(input io.Reader) (bool, error) {
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
