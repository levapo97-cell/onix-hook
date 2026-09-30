// Command onix-hook.
//
// Corre EN TU PC. Lo ejecutan los hooks de Claude Code: lee el payload del hook por stdin,
// arma un RawEvent (contrato de onix-contracts) y lo publica en onix.raw.<sesión> en NATS.
//
// Principios (§1, §5 del plan):
//   - NO bloquea al agente: connect con timeout corto, publica, flush y sale 0 SIEMPRE
//     (aunque NATS falle, para no frenar a Claude Code).
//   - Identificación: ONIX_AGENT_ROLE + ONIX_PROJECT por entorno; rol con fallback.
//   - Conexión saliente (en prod, NATS_URL con TLS + token).
//
// Se cablea en .claude/settings.json para SessionStart, Pre/PostToolUse, Stop, SubagentStop, Notification.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	c "github.com/levapo97-cell/onix-contracts/go/onixcontracts"
)

// claudeHookPayload es el JSON que Claude Code envía por stdin (campos que nos interesan).
type claudeHookPayload struct {
	SessionID     string                 `json:"session_id"`
	HookEventName string                 `json:"hook_event_name"`
	Cwd           string                 `json:"cwd"`
	ToolName      string                 `json:"tool_name"`
	ToolInput     map[string]interface{} `json:"tool_input"`
	ToolResponse  map[string]interface{} `json:"tool_response"`
}

func main() {
	// Fallar en onix-hook NUNCA debe tumbar al agente: recuperamos cualquier panic y salimos 0.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "onix-hook panic (ignorado):", r)
		}
		os.Exit(0)
	}()

	raw, _ := io.ReadAll(io.LimitReader(os.Stdin, 4<<20)) // máx 4 MB
	var p claudeHookPayload
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p)
	}

	ev := buildEvent(p)
	subject := "onix.raw." + nonEmpty(ev.Session, "sin-sesion")

	data, err := json.Marshal(ev)
	if err != nil {
		fmt.Fprintln(os.Stderr, "onix-hook: no se pudo serializar el evento:", err)
		return
	}

	natsURL := getenv("NATS_URL", nats.DefaultURL)
	opts := []nats.Option{nats.Name("onix-hook"), nats.Timeout(2 * time.Second)}
	if tok := os.Getenv("NATS_TOKEN"); tok != "" {
		opts = append(opts, nats.Token(tok))
	}
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "onix-hook: NATS no disponible (evento descartado):", err)
		return
	}
	defer nc.Close()

	if err := nc.Publish(subject, data); err != nil {
		fmt.Fprintln(os.Stderr, "onix-hook: publish falló:", err)
		return
	}
	// Flush con timeout para asegurar el envío sin colgarse.
	if err := nc.FlushTimeout(2 * time.Second); err != nil {
		fmt.Fprintln(os.Stderr, "onix-hook: flush falló:", err)
	}
}

func buildEvent(p claudeHookPayload) c.RawEvent {
	ev := c.RawEvent{
		V:         1,
		Session:   p.SessionID,
		AgentRole: c.AgentRole(getenv("ONIX_AGENT_ROLE", "fullstack")),
		Hook:      c.Hook(nonEmpty(p.HookEventName, "PostToolUse")),
		Ts:        time.Now().UTC(),
		Project:   getenv("ONIX_PROJECT", "onixguard"),
	}
	if p.Cwd != "" {
		ev.Cwd = &p.Cwd
	}
	if p.ToolName != "" {
		t := p.ToolName
		ev.Tool = &t
	}
	if len(p.ToolInput) > 0 {
		ev.Params = p.ToolInput
	}
	// exit_code: algunos tool_response lo traen (p. ej. Bash). Best-effort.
	if code, ok := p.ToolResponse["exit_code"].(float64); ok {
		ci := int64(code)
		ev.Result = &c.RawEventResult{ExitCode: &ci}
	}
	return ev
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func nonEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
