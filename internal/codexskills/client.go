// Package codexskills provides an opt-in repair of Codex's discovered skill catalog.
package codexskills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"
)

type Skill struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Scope    string `json:"scope"`
	Enabled  bool   `json:"enabled"`
	PluginID string `json:"pluginId"`
}

// Client uses only catalog/config methods. It never starts a model thread or turn.
type Client struct {
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	in         io.WriteCloser
	out        *json.Decoder
	requestID  int
	ConfigPath string
}

func Start(ctx context.Context) (*Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--stdio")
	c := &Client{cmd: cmd, cancel: cancel}
	var err error
	c.in, err = cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		_ = c.in.Close()
		cancel()
		return nil, err
	}
	c.out = json.NewDecoder(out)
	if err := cmd.Start(); err != nil {
		_ = c.in.Close()
		_ = out.Close()
		cancel()
		return nil, fmt.Errorf("start Codex app-server (install Codex CLI first): %w", err)
	}
	var init struct {
		CodexHome string `json:"codexHome"`
	}
	err = c.call("initialize", map[string]any{
		"clientInfo": map[string]string{"name": "skillshare_dedup", "version": "1"},
	}, &init)
	if err == nil && !filepath.IsAbs(init.CodexHome) {
		err = fmt.Errorf("Codex did not return an absolute codexHome")
	}
	if err == nil {
		err = json.NewEncoder(c.in).Encode(map[string]any{"method": "initialized"})
	}
	if err != nil {
		c.Close()
		return nil, err
	}
	c.ConfigPath = filepath.Join(init.CodexHome, "config.toml")
	return c, nil
}

func (c *Client) Close() {
	_ = c.in.Close()
	c.cancel()
	_ = c.cmd.Wait()
}

func (c *Client) call(method string, params, result any) error {
	c.requestID++
	if err := json.NewEncoder(c.in).Encode(map[string]any{
		"id": c.requestID, "method": method, "params": params,
	}); err != nil {
		return err
	}
	for {
		var response struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := c.out.Decode(&response); err != nil {
			return fmt.Errorf("Codex %s response: %w", method, err)
		}
		if response.ID != c.requestID {
			continue // Notifications can arrive between responses.
		}
		if response.Error != nil {
			return fmt.Errorf("Codex %s: %s", method, response.Error.Message)
		}
		return json.Unmarshal(response.Result, result)
	}
}

func (c *Client) List(cwd string) ([]Skill, error) {
	var result struct {
		Data []struct {
			Skills []Skill           `json:"skills"`
			Errors []json.RawMessage `json:"errors"`
		} `json:"data"`
	}
	if err := c.call("skills/list", map[string]any{"cwds": []string{cwd}, "forceReload": true}, &result); err != nil {
		return nil, err
	}
	if len(result.Data) != 1 || len(result.Data[0].Errors) != 0 {
		return nil, fmt.Errorf("Codex returned an incomplete skill catalog; repair load errors before deduplicating")
	}
	return result.Data[0].Skills, nil
}

func (c *Client) Disable(path string) error {
	var result struct {
		EffectiveEnabled bool `json:"effectiveEnabled"`
	}
	return c.call("skills/config/write", map[string]any{"path": path, "enabled": false}, &result)
}
