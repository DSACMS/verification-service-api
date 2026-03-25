package veterans

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

func BuildLaunchParam(patientICN string) (string, error) {
	patientICN = normalizeICN(patientICN)
	if patientICN == "" {
		return "", errors.New("patientICN is required")
	}

	b, err := json.Marshal(map[string]string{
		"patient": patientICN,
	})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func normalizeICN(icn string) string {
	return strings.TrimSpace(icn)
}
