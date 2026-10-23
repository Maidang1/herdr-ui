package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/bytedance/herdr-ui/internal/layout"
)

func (s *Session) readTree(ctx context.Context, client *herdr.Client, op Operation) (*layout.Node, string, error) {
	var response struct {
		Layout struct {
			WorkspaceID string      `json:"workspace_id"`
			TabID       string      `json:"tab_id"`
			Root        layout.Node `json:"root"`
		} `json:"layout"`
	}
	if err := client.Call(ctx, "layout.export", map[string]string{"tab_id": op.TabID}, &response); err != nil {
		return nil, "", err
	}
	if response.Layout.TabID != op.TabID || response.Layout.WorkspaceID != op.WorkspaceID {
		return nil, "", &herdr.ProtocolError{Reason: "exported layout scope mismatch"}
	}
	fingerprint, err := layout.Fingerprint(&response.Layout.Root)
	if err != nil {
		return nil, "", &herdr.ProtocolError{Reason: err.Error()}
	}
	return &response.Layout.Root, fingerprint, nil
}

func (s *Session) executeLayout(ctx context.Context, client *herdr.Client, q *queuedOperation) error {
	op := q.operation
	reject := func(err error) error {
		var remote *herdr.RemoteError
		if errors.As(err, &remote) {
			q.complete(Rejected, err.Error(), ScrollInfo{})
			return nil
		}
		q.complete(OutcomeUnknown, err.Error(), ScrollInfo{})
		return err
	}
	switch op.Kind {
	case ExportLayout:
		root, fingerprint, err := s.readTree(ctx, client, op)
		if err != nil {
			return reject(err)
		}
		q.finish(OperationResult{Status: Succeeded, Root: root, Fingerprint: fingerprint})
		return nil
	case SetRatio:
		root, fingerprint, err := s.readTree(ctx, client, op)
		if err != nil {
			// No mutation has been attempted when preparatory export fails.
			q.complete(NotSent, err.Error(), ScrollInfo{})
			return err
		}
		if fingerprint != op.Fingerprint {
			q.complete(StaleTarget, "layout structure changed", ScrollInfo{})
			return nil
		}
		node, err := layout.ResolvePath(root, op.Path)
		if err != nil || node.Type != "split" {
			q.complete(StaleTarget, "split path is no longer valid", ScrollInfo{})
			return nil
		}
		if ctx.Err() != nil {
			q.complete(NotSent, ctx.Err().Error(), ScrollInfo{})
			return nil
		}
		// The extra export is validation, not replay. Each mutation is sent once.
		var changed any
		err = client.Call(ctx, "layout.set_split_ratio", map[string]any{"tab_id": op.TabID, "path": op.Path, "ratio": op.Ratio}, &changed)
		if err != nil {
			return reject(err)
		}
		q.complete(Succeeded, "", ScrollInfo{})
		return nil
	case Split:
		var response struct {
			Pane struct {
				ID        string `json:"pane_id"`
				Workspace string `json:"workspace_id"`
				Tab       string `json:"tab_id"`
			} `json:"pane"`
		}
		err := client.Call(ctx, "pane.split", map[string]any{"workspace_id": op.WorkspaceID, "target_pane_id": op.PaneID, "direction": op.Direction, "ratio": .5, "focus": true}, &response)
		if err != nil {
			return reject(err)
		}
		if response.Pane.ID == "" || response.Pane.Workspace != op.WorkspaceID || response.Pane.Tab != op.TabID {
			return reject(&herdr.ProtocolError{Reason: "split returned an inconsistent target"})
		}
		q.finish(OperationResult{Status: Succeeded, PaneID: response.Pane.ID})
		return nil
	case Neighbor:
		var response struct {
			Neighbor struct {
				PaneID string `json:"pane_id"`
				Next   string `json:"neighbor_pane_id"`
			} `json:"neighbor"`
		}
		err := client.Call(ctx, "pane.neighbor", map[string]string{"pane_id": op.PaneID, "direction": op.Direction}, &response)
		if err != nil {
			return reject(err)
		}
		if response.Neighbor.PaneID != op.PaneID {
			return reject(&herdr.ProtocolError{Reason: "neighbor returned a different source pane"})
		}
		q.finish(OperationResult{Status: Succeeded, PaneID: response.Neighbor.Next})
		return nil
	case Zoom:
		var response struct {
			Zoom struct {
				PaneID string `json:"pane_id"`
			} `json:"zoom"`
		}
		err := client.Call(ctx, "pane.zoom", map[string]string{"pane_id": op.PaneID, "mode": "toggle"}, &response)
		if err != nil {
			return reject(err)
		}
		if response.Zoom.PaneID != op.PaneID {
			return reject(&herdr.ProtocolError{Reason: "zoom returned a different pane"})
		}
		q.complete(Succeeded, "", ScrollInfo{})
		return nil
	}
	q.complete(Rejected, fmt.Sprintf("unsupported layout operation %d", op.Kind), ScrollInfo{})
	return nil
}
