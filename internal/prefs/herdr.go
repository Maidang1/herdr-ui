package prefs

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// HerdrToast reads the [ui.toast] settings from Herdr's config.toml at path.
// Anything missing or invalid comes back empty (DelaySeconds -1), as does an
// unreadable file, so InheritToast keeps its defaults for it. Herdr's
// "terminal" delivery prints into the outer terminal, which this window is,
// so it maps to the in-app card.
func HerdrToast(path string) Toast {
	result := Toast{DelaySeconds: -1}
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	var config struct {
		UI struct {
			Toast struct {
				Delivery     *string `toml:"delivery"`
				Enabled      *bool   `toml:"enabled"`
				DelaySeconds *int64  `toml:"delay_seconds"`
				Herdr        struct {
					Position *string `toml:"position"`
				} `toml:"herdr"`
			} `toml:"toast"`
		} `toml:"ui"`
	}
	if toml.Unmarshal(data, &config) != nil {
		return result
	}
	toast := config.UI.Toast
	switch {
	case toast.Delivery != nil:
		switch *toast.Delivery {
		case "off", "herdr", "system":
			result.Delivery = *toast.Delivery
		case "terminal":
			result.Delivery = "herdr"
		}
	case toast.Enabled != nil:
		// Herdr's legacy switch: on was the in-app toast.
		result.Delivery = "off"
		if *toast.Enabled {
			result.Delivery = "herdr"
		}
	}
	if toast.Herdr.Position != nil && validPosition(*toast.Herdr.Position) {
		result.Position = *toast.Herdr.Position
	}
	if toast.DelaySeconds != nil && *toast.DelaySeconds >= 0 && *toast.DelaySeconds <= maxToastDelay {
		result.DelaySeconds = int(*toast.DelaySeconds)
	}
	return result
}
