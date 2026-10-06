package box

import (
	"bytes"
	"net/http"
	"os"
	"strings"

	"github.com/sean-brydon/berthd/internal/integrations"
)

// Integrations are the hooks agent CLIs on this box run to report their
// needs-you, working and done states, and berth's skills. berthd install
// sets them up for the CLIs it finds; these let the app add them for a CLI
// installed since.

// IntegrationTool is one agent CLI and whether berth's hooks are in its
// settings for this box's user.
type IntegrationTool struct {
	integrations.Tool
	Present bool `json:"present"`
	Hooked  bool `json:"hooked"`
}

// IntegrationsReport lists the agent CLIs berth has integrations for.
type IntegrationsReport struct {
	Tools []IntegrationTool `json:"tools"`
	// Output is what an install did, in berthd integrations install's words.
	Output string `json:"output,omitempty"`
}

func integrationsReport() (IntegrationsReport, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return IntegrationsReport{}, err
	}
	out := IntegrationsReport{Tools: []IntegrationTool{}}
	for _, t := range integrations.Tools {
		out.Tools = append(out.Tools, IntegrationTool{Tool: t, Present: t.Present(home), Hooked: t.Hooked(home)})
	}
	return out, nil
}

func (b *Box) listIntegrations(w http.ResponseWriter, r *http.Request) error {
	rep, err := integrationsReport()
	if err != nil {
		return err
	}
	writeJSON(w, rep)
	return nil
}

// installIntegrations installs one tool's hooks and skills for this box's
// user, as `berthd integrations install TOOL` does.
func (b *Box) installIntegrations(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Tool string `json:"tool"`
	}
	if err := decode(r, &req); err != nil {
		return err
	}
	if _, ok := integrations.ToolByID(req.Tool); !ok {
		return badRequest("unknown tool %q; use %s or all", req.Tool, strings.Join(integrations.AllTools, ", "))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	self, err := b.self()
	if err != nil {
		return err
	}
	data := map[string]any{"tool": req.Tool}
	if err := b.before(r, "integrations.install", data); err != nil {
		return err
	}
	var out bytes.Buffer
	if err := integrations.InstallTool(home, req.Tool, self, &out); err != nil {
		return badRequest("%v", err)
	}
	b.publish(r, "integrations.installed", data)
	rep, err := integrationsReport()
	if err != nil {
		return err
	}
	rep.Output = strings.TrimSpace(out.String())
	writeJSON(w, rep)
	return nil
}

// self is the berthd binary serving this box, which hooks and wrappers run.
func (b *Box) self() (string, error) {
	if b.Update != nil && b.Update.Executable != "" {
		return b.Update.Executable, nil
	}
	return os.Executable()
}
