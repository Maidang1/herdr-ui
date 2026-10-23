// Package layout interprets Herdr's exported tree. Paths use false=first,
// true=second; split IDs from the rectangle snapshot are not paths.
package layout

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	visited := map[*Node]bool{}
	var walk func(*Node, []bool) error
	walk = func(n *Node, path []bool) error {
		if n == nil {
			return fmt.Errorf("missing layout child")
		}
		if visited[n] {
			return fmt.Errorf("layout contains reused or cyclic node")
		}
		visited[n] = true
		switch n.Type {
		case "pane":
			if n.First != nil || n.Second != nil {
				return fmt.Errorf("pane has layout children")
			}
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

// Clone returns an independent tree. Sharing/cycles are preserved for malformed
// inputs so validation can reject them without infinite recursion.
func Clone(root *Node) *Node {
	seen := map[*Node]*Node{}
	var clone func(*Node) *Node
	clone = func(n *Node) *Node {
		if n == nil {
			return nil
		}
		if c, ok := seen[n]; ok {
			return c
		}
		c := *n
		seen[n] = &c
		c.First = clone(n.First)
		c.Second = clone(n.Second)
		return &c
	}
	return clone(root)
}

// Fingerprint identifies pane identities and topology, excluding split ratios.
func Fingerprint(root *Node) (string, error) {
	if _, err := SplitPaths(root); err != nil {
		return "", err
	}
	c := Clone(root)
	var clear func(*Node)
	clear = func(n *Node) {
		n.Ratio = 0
		if n.Type == "pane" {
			n.Direction = ""
		} else {
			n.PaneID = ""
			clear(n.First)
			clear(n.Second)
		}
	}
	clear(c)
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func FindPath(root *Node, paneID string) ([]bool, error) {
	if _, err := SplitPaths(root); err != nil {
		return nil, err
	}
	var find func(*Node, []bool) ([]bool, bool)
	find = func(n *Node, path []bool) ([]bool, bool) {
		if n.Type == "pane" {
			return path, n.PaneID == paneID
		}
		if p, ok := find(n.First, append(append([]bool{}, path...), false)); ok {
			return p, true
		}
		return find(n.Second, append(append([]bool{}, path...), true))
	}
	if path, ok := find(root, []bool{}); ok {
		return path, nil
	}
	return nil, fmt.Errorf("pane %q absent from layout", paneID)
}

// SplitForPane prefers the requested edge, then falls back to its opposite.
func SplitForPane(root *Node, paneID, direction string) ([]bool, error) {
	axis := ""
	switch direction {
	case "left", "right":
		axis = "right"
	case "up", "down":
		axis = "down"
	default:
		return nil, fmt.Errorf("invalid resize direction %q", direction)
	}
	path, err := FindPath(root, paneID)
	if err != nil {
		return nil, err
	}
	// Prefer the requested edge. Only fall back to the opposite edge when
	// there is no ancestor split on that side, matching Herdr resize rules.
	wantSecond := direction == "left" || direction == "up"
	for _, second := range []bool{wantSecond, !wantSecond} {
		for depth := len(path) - 1; depth >= 0; depth-- {
			n, err := ResolvePath(root, path[:depth])
			if err != nil {
				return nil, err
			}
			if n.Direction == axis && path[depth] == second {
				return append([]bool{}, path[:depth]...), nil
			}
		}
	}
	return nil, fmt.Errorf("pane %q has no split on %s axis", paneID, axis)
}
