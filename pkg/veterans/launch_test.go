package veterans

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestBuildLaunchParam_RequiresICN(t *testing.T) {
	_, err := BuildLaunchParam("   ")
	if !errors.Is(err, ErrICNRequired) {
		t.Fatalf("expected ErrICNRequired, got %v", err)
	}
}

func TestBuildLaunchParam_EncodesNormalizedICN(t *testing.T) {
	launch, err := BuildLaunchParam("  12345  ")
	if err != nil {
		t.Fatalf("BuildLaunchParam: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(launch)
	if err != nil {
		t.Fatalf("decode launch: %v", err)
	}

	if !strings.Contains(string(decoded), `"patient":"12345"`) {
		t.Fatalf("unexpected launch payload: %s", string(decoded))
	}
}
