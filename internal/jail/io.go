package jail

import (
	"os"
	"strconv"
	"strings"
)

func readUint64File(p string) (uint64, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}
