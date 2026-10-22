package ui

// sampleSession is the session the design file shows.
//
// It is here so that the window has something to draw before the daemon is
// connected: the same session, in the same state, as the board of the
// design.
func sampleSession() Session {
	return Session{
		Title: "Herdr",
		Spaces: []Space{
			{
				Name:     "deepseek-harness",
				Selected: true,
				Branch:   "master",
				Ports:    ":8080  :8081  :9200",
				Agents: []Agent{
					{Name: "daimon", Branch: "main", Ports: ":3180"},
				},
			},
			{
				Name:   "cli 可视化项目",
				Branch: "feat/agent-plugin",
				Ports:  ":50889  :52325",
			},
			{
				Name:   "dsh",
				Branch: "main",
				Ports:  ":4310  :4331  :4340  :6268",
			},
			{
				Name:   "coze-loop-frontend",
				Branch: "release/20261021",
				Ports:  ":58730",
			},
			{
				Name:   "fornax-cli",
				Branch: "master",
			},
		},
		Groups: []Group{
			{
				Name:  "deepseek-harness",
				Count: 1,
				Agents: []Agent{
					{Name: "grok", State: "idle"},
				},
			},
			{
				Name:  "dsh",
				Count: 2,
				Agents: []Agent{
					{Name: "codex", State: "idle"},
				},
			},
			{
				Name:  "dsh",
				Count: 4,
				Agents: []Agent{
					{Name: "copilot", State: "idle"},
				},
			},
		},
		Tabs: []Tab{
			{Number: 1, Label: "1", Dot: ToneSuccess},
			{Number: 2, Label: "2", Dot: ToneWarning, Active: true},
		},
		Lines: [][]Segment{
			{
				{Text: "(base)", Role: RoleSecondary},
			},
			{
				{Text: "deepseek-harness", Role: RoleGreen, Weight: 700},
				{Text: " on ", Role: RoleSecondary},
				{Text: "⑂ master", Role: RoleYellow},
				{Text: " via ", Role: RoleSecondary},
				{Text: "●", Role: RoleGreen},
				{Text: " v24.15.0", Role: RoleGreen, Weight: 700},
			},
			{
				{Text: "❯ ", Role: RoleYellow},
				{Text: "pwd", Role: RoleGreen},
			},
			{
				{Text: "/Users/bytedance/codes/open-source/deepseek-harness", Role: RolePrimary},
			},
			{
				{Text: "(base)", Role: RoleSecondary},
			},
			{
				{Text: "deepseek-harness", Role: RoleGreen, Weight: 700},
				{Text: " on ", Role: RoleSecondary},
				{Text: "⑂ master", Role: RoleYellow},
				{Text: " ", Role: RoleSecondary},
				{Text: "[+?]", Role: RoleRed},
				{Text: " via ", Role: RoleSecondary},
				{Text: "●", Role: RoleGreen},
				{Text: " v24.15.0", Role: RoleGreen, Weight: 700},
			},
		},
		Meters: []Meter{
			{Icon: "activity", Value: 0.27, Label: "5h 27% used 0m · 5h 86% used 3h 21m", Tone: ToneSuccess},
			{Icon: "sparkles", Value: 0.78, Label: "wk 78% used 5d 3h", Tone: ToneWarning},
			{Icon: "sun", Value: 0.34, Label: "5h 9% used 0m · wk 34% used 4d 0h", Tone: ToneNeutral},
		},
		Ports:  ":8080 :8081 :9200",
		CPU:    0.48,
		Memory: 0.83,
		Date:   "10-20 11:24",
	}
}
