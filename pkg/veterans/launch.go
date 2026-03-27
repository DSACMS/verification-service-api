package veterans

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

var ErrICNRequired = errors.New("'icn' is required")

func BuildLaunchParam(icn string) (string, error) {
	icn = normalizeICN(icn)
	if icn == "" {
		return "", ErrICNRequired
	}

	b, err := json.Marshal(map[string]string{
		"patient": icn,
	})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func normalizeICN(icn string) string {
	return strings.TrimSpace(icn)
}
