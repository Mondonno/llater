# LLM Debate

Turn your notes into a ruthless AI debate — then get a blunt summary of blind spots, opportunities, and deadly assumptions.

Two models. One claims, one defends. Both remember context. You get the punchline.

See how they deteriorate in their arguments where entropy is all the time raising.

---

## Why this gets people talking
- Spicy by design: a Challenger aggressively tears ideas apart; a Defender has your back.
- Context that sticks: debate history is preserved across rounds, so arguments escalate, not reset.
- Local-first: runs against your Ollama models — fast, private, and hackable.
- Shareable outcomes: the final output is a concise, high-signal summary you can post.

> Perfect for startup ideas, strategy memos, research notes, journaling, and "am I missing something huge?" moments.

---

## Quick start (TL;DR)
Prereqs:
- Go 1.24+
- Ollama installed and running (on macOS: `brew install ollama && ollama serve`)
- At least one model pulled (default: `llama3.1`)

Pull a model:
```bash
ollama pull llama3.1
```

Run directly:
```bash
go run . \
  --input ./notes.md \
  --output ./report.md \
  --rounds 3
```

Or build a binary:
```bash
go build -o llmdebate
./llmdebate --input ./notes.md --output ./report.md --rounds 3
```

What you’ll get:
- Live progress in your terminal while models generate.
- A terse, opinionated summary written to `--output` (the full back-and-forth is logged to stdout).

---

## Features at a glance
- Two-role debate: Challenger vs. Defender
- Model flexibility: use the same model for both roles or mix and match
- Context-preserving history across rounds (arguments compound)
- Round-based or duration-based runs
- Simple, overridable role prompts
- Final summary focused on: blind spots, opportunities, deadly assumptions

---

## Usage
CLI flags:
- `--input` (required): path to your input notes/claim
- `--output` (required): where to save the final summary
- `--rounds`: number of debate rounds (mutually exclusive with `--duration`)
- `--duration`: total wall-clock time, e.g. `3m`, `1h` (auto-estimates rounds; mutually exclusive with `--rounds`)
- `--challenger`: model name for the Challenger (default: `llama3.1`)
- `--defender`: model name for the Defender (default: `llama3.1`)
- `--challenger-prompt`: custom system prompt or path to a prompt file for the Challenger
- `--defender-prompt`: custom system prompt or path to a prompt file for the Defender

Examples:
```bash
# Run 5 rounds with defaults
llmdebate --input notes.md --output report.md --rounds 5

# Time-box to 3 minutes (auto-estimate rounds)
llmdebate --input notes.md --output report.md --duration 3m

# Mix models and use custom role prompts
llmdebate \
  --input notes.md \
  --output report.md \
  --challenger mistral:latest \
  --defender llama3.1 \
  --challenger-prompt ./sessions/challenger_system.md \
  --defender-prompt ./sessions/defender_system.md
```

Notes:
- If you omit both `--rounds` and `--duration`, it will run indefinitely — press Ctrl+C to stop.
- The summary uses the Challenger’s model for the final pass by default.

---

## Example output (summary)
```
Top blind spots
- Assumes early adopters will tolerate onboarding friction; no mitigation plan.
- Competitive response underestimated; no defensibility beyond speed.

Opportunities
- Partner with X to piggyback on existing distribution.
- Narrow ICP to Y to increase win-rate and shorten sales cycle.

Deadly assumption
- Users will consistently provide high-quality input; if wrong, core value collapses.
```

---

## Under the hood
- Language: Go
- CLI: Cobra
- LLM runtime: Ollama client
- UX: progress bar while generating
- Context: trims history to stay within limits, but preserves debate memory across rounds

High-level flow:
1) Load input → 2) Alternate Challenger/Defender across N rounds with preserved history → 3) Summarize into a blunt report.

---

## Tips for better debates
- Seed with concrete notes, decisions, or claims; vague inputs lead to vague fights.
- Use a creative Challenger and a conservative Defender for spicy but grounded exchanges.
- Tune prompts per role (see `sessions/challenger_system.md` and `sessions/defender_system.md`).
- Time-box early explorations with `--duration 2-5m` to iterate fast.

---

## Develop
Run tests:
```bash
go test ./...
```

Hack on it:
- Swap models via flags (works with any Ollama model you’ve pulled)
- Adjust temperature/top-p/etc. inside `main.go` if you want a different default vibe

---

## Share your best debates
If a summary exposed a brutal blind spot (or saved you from a bad launch), post the TL;DR with your redacted input. That’s the good stuff people upvote.

Stay safe and respectful when sharing. Models hallucinate; you’re the judge.

