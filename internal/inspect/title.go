package inspect

import "regexp"

// TitleRule recognises a coding agent by the terminal title (OSC 0/2) it
// sets. It is the fallback for when the agent's process isn't in our
// process table: over ssh or mosh, inside docker exec, and so on.
type TitleRule struct {
	Agent string
	// Status the title implies: working, needs-input, idle, or "" when
	// it says nothing about it.
	Status string
	// Start marks titles distinctive enough to say on their own that the
	// agent runs. The others (e.g. a bare spinner glyph) only keep an agent
	// already recognised, so other programs' titles aren't mistaken for one.
	Start bool
	Re    *regexp.Regexp
}

// TitleRules are matched in order; the first match wins.
var TitleRules = []TitleRule{
	// Claude Code: "✳ Claude Code" at start, "✳ <topic>" when idle or
	// waiting, and a half-moon spinner "◐ <topic>" / "◑ <topic>" while it
	// works (recorded from claude 2.1). Older versions spun braille dots.
	{Agent: "claude", Status: "idle", Start: true, Re: regexp.MustCompile(`^✳\s`)},
	{Agent: "claude", Status: "working", Start: true, Re: regexp.MustCompile(`^[◐◑◒◓⠁-⣿]\s+Claude Code$`)},
	{Agent: "claude", Status: "working", Re: regexp.MustCompile(`^[◐◑◒◓⠁-⣿]\s+\S`)},
	{Agent: "claude", Status: "", Start: true, Re: regexp.MustCompile(`^Claude Code$`)},
	// Gemini CLI: "◇  Ready (dir)", "✦  <thought> (dir)", "✋  Action
	// Required (dir)"; older versions "Gemini - dir".
	{Agent: "gemini", Status: "needs-input", Start: true, Re: regexp.MustCompile(`^✋\s+Action Required \(.*\)$`)},
	{Agent: "gemini", Status: "idle", Start: true, Re: regexp.MustCompile(`^◇\s+Ready \(.*\)$`)},
	{Agent: "gemini", Status: "working", Re: regexp.MustCompile(`^✦\s+.*\(.*\)$`)},
	{Agent: "gemini", Status: "", Start: true, Re: regexp.MustCompile(`^Gemini - \S`)},
	// Codex: "<dir>" when idle, "⠋ <dir>" spinning while it works, and
	// "[ ! ] Action Required | <dir>" blinking with "[ . ]" while it waits
	// for an answer (recorded from codex-cli 0.163). Neither is distinctive
	// enough on its own: Codex is known by its process or its OSC 7501 state.
	{Agent: "codex", Status: "needs-input", Re: regexp.MustCompile(`^\[ [!.] \] Action Required\b`)},
	{Agent: "codex", Status: "working", Re: regexp.MustCompile(`^[⠁-⣿]\s+\S`)},
	// OpenCode: "OpenCode" at start, "OC | <session title>" after
	// (opencode 1.19); nothing about its status.
	{Agent: "opencode", Status: "", Start: true, Re: regexp.MustCompile(`^OpenCode$`)},
	{Agent: "opencode", Status: "", Start: true, Re: regexp.MustCompile(`^OC \| \S`)},
	// pi: "π - <dir>" from start until it exits (pi 1.3, recorded in a
	// pty); it sets the title once and never spins it, so it says nothing
	// about its status.
	{Agent: "pi", Status: "", Start: true, Re: regexp.MustCompile(`^π - \S`)},
}

// TitleAgent reads a terminal title. current is the agent already seen in
// this terminal ("" if none): titles that aren't distinctive on their own
// only count for that same agent. ok is false when the title says no agent
// runs (any other title, including the empty one agents set on exit).
func TitleAgent(title, current string) (agent, status string, ok bool) {
	for _, r := range TitleRules {
		if r.Re.MatchString(title) && (r.Start || r.Agent == current) {
			return r.Agent, r.Status, true
		}
	}
	return "", "", false
}
