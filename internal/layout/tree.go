// Package layout interprets Herdr's exported tree. Paths use false=first,
// true=second; split IDs from the rectangle snapshot are not paths.
package layout

import (
	"fmt"
	"math"
)

type Node struct {
	Type      string  `json:"type"`
	PaneID    string  `json:"pane_id,omitempty"`
	Direction string  `json:"direction,omitempty"`
	Ratio     float64 `json:"ratio,omitempty"`
	First     *Node   `json:"first,omitempty"`
	Second    *Node   `json:"second,omitempty"`
}

func ResolvePath(root *Node, path []bool) (*Node, error) {
	n := root
	for _, second := range path {
		if n == nil || n.Type != "split" {
			return nil, fmt.Errorf("path traverses missing split")
		}
		if second {
			n = n.Second
		} else {
			n = n.First
		}
	}
	if n == nil {
		return nil, fmt.Errorf("path resolves to missing node")
	}
	return n, nil
}

func SplitPaths(root *Node) ([][]bool, error) {
	var paths [][]bool
	seen := map[string]bool{}
	var walk func(*Node, []bool) error
	walk = func(n *Node, path []bool) error {
		if n == nil {
			return fmt.Errorf("missing layout child")
		}
		switch n.Type {
		case "pane":
			if n.PaneID == "" || seen[n.PaneID] {
				return fmt.Errorf("missing or duplicate pane ID %q", n.PaneID)
			}
			seen[n.PaneID] = true
		case "split":
			if (n.Direction != "right" && n.Direction != "down") || math.IsNaN(n.Ratio) || n.Ratio < 0.1 || n.Ratio > 0.9 {
				return fmt.Errorf("invalid split")
			}
			paths = append(paths, append([]bool{}, path...))
			if err := walk(n.First, append(append([]bool{}, path...), false)); err != nil {
				return err
			}
			return walk(n.Second, append(append([]bool{}, path...), true))
		default:
			return fmt.Errorf("unknown layout node %q", n.Type)
		}
		return nil
	}
	err := walk(root, nil)
	return paths, err
}
