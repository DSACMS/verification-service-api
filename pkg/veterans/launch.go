package veterans

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

func BuildLaunchParam(patientICN string) (string, error) {
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
