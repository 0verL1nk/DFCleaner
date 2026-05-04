# Core Beliefs

These are the foundational principles that guide all design decisions in DFCleaner.

## 1. Users Own Their Data

DFCleaner runs locally. All data stays on the user's machine. The only outbound network traffic is LLM API calls that the user explicitly configured. No telemetry, no analytics, no phone-home.

## 2. AI Advises, User Decides

The AI Agent can scan, analyze, and suggest. It cannot delete, move, or modify. This isn't a policy — it's an architectural constraint enforced at the tool registration level. The Agent physically lacks the tools to do harm.

## 3. Visual Understanding Before Action

Users should understand their disk usage at a glance before making decisions. The Treemap visualization, color-coded risk levels, and inline AI explanations serve this goal. Text-only analysis isn't enough.

## 4. Boring Technology, Novel Application

We use proven, well-tested tools (Go, React, SQLite, Eino, d3). The innovation is in how they're combined — an AI-powered disk cleanup experience — not in any individual technology choice.

## 5. Iterate Based on Real Usage

We don't over-design upfront. AI analysis strategy, batch sizes, trigger timing — these should be validated with real end-to-end testing and real user feedback. Ship the basic flow, measure, improve.

## 6. Global by Default

i18n infrastructure from day one. Dark/light theme from day one. Cross-platform from day one. These things are cheap to build in but expensive to retrofit.

## 7. Simple > Complete

Three views, not five. One Agent, not a multi-agent system. Trash by default, not a complex undo system. The simplest solution that solves the user's problem is the right one.

## 8. Cost Awareness

Users bring their own LLM API key. We respect this by:
- Sending only metadata by default (not file content)
- Batching analysis to minimize API calls
- Summarizing tool outputs before sending to LLM
- Supporting free local models (Ollama)
