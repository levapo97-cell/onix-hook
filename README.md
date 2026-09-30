# onix-hook

Binario **Go** que corre **en tu PC**. Lo ejecutan los hooks de Claude Code: arma el evento crudo y lo publica en `onix.raw.<sesión>` por una **conexión saliente** a NATS (TLS) en el VPS. Sin lógica pesada; no bloquea al agente.

> **Estado (Fase 1 ✅):** implementado. Lee el payload del hook por stdin, arma el `RawEvent` y lo publica en `onix.raw.<sesión>`. No bloquea al agente (connect con timeout, sale 0 siempre). Verificado E2E (2026-09-30): el evento aparece en la web y en Postgres en <1s.

## Uso rápido (build + prueba)
```bash
go build -o onix-hook ./cmd/onix-hook
echo '{"session_id":"demo","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}' \
  | NATS_URL=nats://localhost:4222 ONIX_PROJECT=onixguard ONIX_AGENT_ROLE=fullstack ./onix-hook
```

---

## Por qué existe y dónde corre

Es el **único componente que corre en tu casa**, no en el VPS. Así la telemetría sale de tu PC sin abrir puertos: solo hace conexiones **salientes**.

```mermaid
flowchart LR
  CC["Claude Code (agente)"] -->|"hook: stdin JSON"| HOOK["onix-hook (binario)"]
  HOOK -->|"RawEvent → onix.raw.&lt;sesión&gt;<br/>NATS TLS saliente"| VPS[("NATS en el VPS")]
```

## Cómo funcionará

- Se cablea en `.claude/settings.json` de cada agente para los hooks: `SessionStart`, `PreToolUse`, `PostToolUse`, `Stop`, `SubagentStop`, `Notification`.
- Recibe el payload del hook por **stdin**, lo transforma en un **RawEvent** (ver `onix-contracts`) y lo publica en NATS.
- **Identificación del agente:** lee `ONIX_AGENT_ROLE` y `ONIX_PROJECT` del entorno; si faltan, infiere el rol de la skill activa.
- **No bloquea:** publica y sale rápido; si NATS no responde, falla en silencio para no frenar al agente.
- Config por entorno: `NATS_URL` (TLS, saliente al VPS), `NATS_TOKEN` (token de máquina), `ONIX_AGENT_ROLE`, `ONIX_PROJECT`.

## Contratos

- **Produce:** `RawEvent` en `onix.raw.<claude_session_id>` → lo consume `onix-ingestor`.
- **Importa:** `github.com/levapo97-cell/onix-contracts/go/onixcontracts`.

## Instalación (futuro, Fase 1)

```bash
go build -o onix-hook ./cmd/onix-hook
# instalar en el PATH y referenciar desde los hooks de Claude Code
```

---

*Parte de OnixGuard. Ver el plan en `OnixGuard/docs/PLAN.md` §1 (componentes) y §5 (hooks).*
