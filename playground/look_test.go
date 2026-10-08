package playground

import (
	"os"
	"testing"
)

func TestLookCapture(t *testing.T) {
	if os.Getenv("PLAYGROUND_LOOK") == "" {
		t.Skip("set PLAYGROUND_LOOK=1 to write captures")
	}
	for _, dark := range []bool{false, true} {
		SetupText(1)
		s := NewState(1280, 800, dark)
		buf := make([]byte, 1280*800*4)
		s.Draw(buf)
		name := "light.png"
		if dark {
			name = "dark.png"
		}
		savePNG(t, name, buf, 1280, 800)
	}
}
