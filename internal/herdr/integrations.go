package herdr

import (
	"context"
	"fmt"
)

// Integration describes the server's agent command and hook installation.
// Availability and installation are independent: hooks can remain installed
// after the agent executable has been removed.
type Integration struct {
	Target    string `json:"target"`
	Label     string `json:"label"`
	Command   string `json:"command"`
	Available bool   `json:"available"`
	State     string `json:"state"`
}

func ListIntegrations(ctx context.Context, target Target) ([]Integration, error) {
	var result struct {
		Type         string        `json:"type"`
		Integrations []Integration `json:"integrations"`
	}
	if err := Call(ctx, target, "integration.list", nil, &result); err != nil {
		return nil, err
	}
	if result.Type != "integration_list" || result.Integrations == nil {
		return nil, fmt.Errorf("Herdr returned an invalid integration list")
	}
	return result.Integrations, nil
}

// ChangeIntegration uses Herdr's installer so its own version checks and
// configuration merging apply. Mutations are sent once, without retries.
func ChangeIntegration(ctx context.Context, target Target, action, agent string) ([]string, error) {
	if (action != "install" && action != "uninstall") || agent == "" {
		return nil, fmt.Errorf("invalid integration action or target")
	}
	var result struct {
		Type    string `json:"type"`
		Target  string `json:"target"`
		Details *struct {
			Messages []string `json:"messages"`
		} `json:"details"`
	}
	if err := Call(ctx, target, "integration."+action, map[string]string{"target": agent}, &result); err != nil {
		return nil, err
	}
	if result.Type != "integration_"+action || result.Target != agent || result.Details == nil || result.Details.Messages == nil {
		return nil, fmt.Errorf("Herdr returned an invalid integration result; refresh to check the installed state")
	}
	return result.Details.Messages, nil
}
