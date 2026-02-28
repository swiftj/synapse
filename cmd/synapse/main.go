// Synapse - The shared nervous system for Vibe Coders and their Agents.
//
// A lightweight, local-first, Git-backed issue tracker designed to serve
// as persistent "long-term memory" for AI agents.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/swiftj/synapse/internal/mcp"
	"github.com/swiftj/synapse/internal/skill"
	"github.com/swiftj/synapse/internal/storage"
	"github.com/swiftj/synapse/internal/view"
	"github.com/swiftj/synapse/pkg/types"
)

const version = "1.0.7"

var jsonOutput bool

// jsonOut writes v as indented JSON to stdout.
func jsonOut(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// extractGlobalFlags scans os.Args for --json, sets jsonOutput, and strips
// the flag so per-command parsers don't see it.
func extractGlobalFlags() {
	filtered := os.Args[:0]
	for _, arg := range os.Args {
		if arg == "--json" {
			jsonOutput = true
		} else {
			filtered = append(filtered, arg)
		}
	}
	os.Args = filtered
}

func main() {
	extractGlobalFlags()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "init":
		cmdInit(args)
	case "add":
		cmdAdd(args)
	case "list", "ls":
		cmdList(args)
	case "ready":
		cmdReady(args)
	case "get":
		cmdGet(args)
	case "claim":
		cmdClaim(args)
	case "done":
		cmdDone(args)
	case "all-done":
		cmdDoneAll()
	case "delete", "rm":
		cmdDelete(args)
	case "breadcrumb", "bc":
		cmdBreadcrumb(args)
	case "skill":
		cmdSkill(args)
	case "serve":
		cmdServe()
	case "view":
		cmdView(args)
	case "version", "-v", "--version":
		if jsonOutput {
			jsonOut(map[string]string{"version": version})
			return
		}
		fmt.Printf("synapse v%s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Synapse - The shared nervous system for Vibe Coders and their Agents.

Usage:
  synapse [--json] <command> [arguments]

Global Flags:
  --json            Output structured JSON (works with any command)

Commands:
  init              Initialize .synapse directory in current project
      --git         Also stage memory.jsonl for commit
  add <title>       Create a new synapse task
      --blocks N    Block on synapse N (can repeat)
      --parent N    Set parent synapse ID
      --assignee X  Assign to role (e.g., @qa, @coder)
  list, ls          List all synapses
      --status X    Filter by status (open, in-progress, blocked, review, done)
      --limit N     Limit output to N tasks (default 20, 0 for unlimited)
      --summary     Condensed output (default)
      --full        Show all fields for each task
  ready             List ready (unblocked, open) tasks
  get <id>          Get details of a specific synapse
  claim <id>        Mark synapse as in-progress
  done <id>         Mark synapse as done
  all-done          Mark all tasks as done (cleanup command)
  delete, rm <id>   Delete a synapse task
      --all         Delete all tasks
      --done        Delete all completed tasks (cleanup)
  breadcrumb, bc    Manage breadcrumbs (persistent key-value storage)
      set <key> <value>   Set a breadcrumb value
          --task-id N     Link to task ID
      get <key>           Get a breadcrumb value
      list [prefix]       List breadcrumbs (optionally filter by prefix)
      delete <key>        Delete a breadcrumb
  skill             Manage agentic skill installations
      install <agent>   Install skill for an agent
          --level L     Install level: user or project (default: project)
      uninstall <agent> Remove skill for an agent
          --level L     Install level: user or project (default: project)
      list              Show installation status for all agents
      update [agent]    Update installed skill(s)
          --level L     Install level: user or project (default: project)
      show              Print the embedded SKILL.md content
  serve             Start MCP server (JSON-RPC over stdio)
  view              Start visualization web server
      --port N      Port to listen on (default: 8080)
  version           Print version
  help              Print this help message

Examples:
  synapse init
  synapse --json add "Fix login bug" --blocks 4 --parent 2
  synapse --json ready
  synapse claim 5
  synapse --json done 5`)
}

func getStore() *storage.JSONLStore {
	store := storage.NewJSONLStore(storage.DefaultDir)
	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "error loading store: %v\n", err)
		os.Exit(1)
	}
	return store
}

func saveStore(store *storage.JSONLStore) {
	if err := store.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "error saving store: %v\n", err)
		os.Exit(1)
	}
}

func cmdInit(args []string) {
	// Parse --git flag
	stageMemory := false
	for _, arg := range args {
		if arg == "--git" {
			stageMemory = true
		}
	}

	store := storage.NewJSONLStore(storage.DefaultDir)
	result, err := store.InitWithOptions(stageMemory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		jsonOut(result)
		return
	}

	fmt.Println("Initialized .synapse directory")
	if result.MemoryCreated {
		fmt.Println("  ✓ Created memory.jsonl")
	} else {
		fmt.Println("  - memory.jsonl already exists")
	}

	if result.GitRepoDetected {
		if result.MemoryStaged {
			fmt.Println("  ✓ Staged .synapse/memory.jsonl for commit")
		} else if stageMemory {
			fmt.Println("  - Could not stage memory.jsonl")
		}
	} else {
		fmt.Println("  - Not a Git repository (skipping Git integration)")
	}
}

func cmdAdd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: title required")
		fmt.Fprintln(os.Stderr, "usage: synapse add <title> [--blocks N] [--parent N] [--assignee X]")
		os.Exit(1)
	}

	var title string
	var blocks []int
	var parentID int
	var assignee string

	// Parse arguments
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--blocks" && i+1 < len(args):
			i++
			id, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid blocker ID: %s\n", args[i])
				os.Exit(1)
			}
			blocks = append(blocks, id)
		case arg == "--parent" && i+1 < len(args):
			i++
			id, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid parent ID: %s\n", args[i])
				os.Exit(1)
			}
			parentID = id
		case arg == "--assignee" && i+1 < len(args):
			i++
			assignee = args[i]
		case !strings.HasPrefix(arg, "--"):
			if title == "" {
				title = arg
			} else {
				title = title + " " + arg
			}
		default:
			fmt.Fprintf(os.Stderr, "error: unknown flag or missing value: %s\n", arg)
			os.Exit(1)
		}
		i++
	}

	if title == "" {
		fmt.Fprintln(os.Stderr, "error: title required")
		os.Exit(1)
	}

	store := getStore()
	syn, err := store.Create(title)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	syn.BlockedBy = blocks
	syn.ParentID = parentID
	syn.Assignee = assignee

	if len(blocks) > 0 {
		syn.Status = types.StatusBlocked
	}

	saveStore(store)

	if jsonOutput {
		jsonOut(syn)
		return
	}

	fmt.Printf("Created synapse #%d: %s\n", syn.ID, syn.Title)
	if len(blocks) > 0 {
		fmt.Printf("  Blocked by: %v\n", blocks)
	}
	if parentID > 0 {
		fmt.Printf("  Parent: #%d\n", parentID)
	}
	if assignee != "" {
		fmt.Printf("  Assignee: %s\n", assignee)
	}
}

func cmdList(args []string) {
	var statusFilter string
	var fullOutput bool
	limit := 20 // default limit

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 < len(args) {
				i++
				statusFilter = args[i]
			}
		case "--limit":
			if i+1 < len(args) {
				i++
				n, err := strconv.Atoi(args[i])
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: invalid limit: %s\n", args[i])
					os.Exit(1)
				}
				limit = n
			}
		case "--full":
			fullOutput = true
		case "--summary":
			fullOutput = false // explicit summary mode (default)
		}
	}

	store := getStore()
	var synapses []*types.Synapse

	if statusFilter != "" {
		status := types.Status(statusFilter)
		if !status.IsValid() {
			fmt.Fprintf(os.Stderr, "error: invalid status: %s\n", statusFilter)
			fmt.Fprintf(os.Stderr, "valid statuses: open, in-progress, blocked, review, done\n")
			os.Exit(1)
		}
		synapses = store.ByStatus(status)
	} else {
		synapses = store.All()
	}

	totalCount := len(synapses)

	// Apply limit (0 means unlimited)
	if limit > 0 && len(synapses) > limit {
		synapses = synapses[:limit]
	}

	if jsonOutput {
		jsonOut(synapses)
		return
	}

	if len(synapses) == 0 {
		fmt.Println("No synapses found")
		return
	}

	if totalCount > len(synapses) {
		fmt.Printf("Showing %d of %d synapse(s) (use --limit 0 for all):\n\n", len(synapses), totalCount)
	} else {
		fmt.Printf("Found %d synapse(s):\n\n", len(synapses))
	}

	for _, syn := range synapses {
		if fullOutput {
			printSynapseDetailed(syn)
			fmt.Println()
		} else {
			printSynapse(syn)
		}
	}
}

func cmdReady(args []string) {
	store := getStore()
	ready := store.Ready()

	if jsonOutput {
		jsonOut(ready)
		return
	}

	if len(ready) == 0 {
		fmt.Println("No ready tasks")
		return
	}

	fmt.Printf("Ready tasks (%d):\n\n", len(ready))
	for _, syn := range ready {
		printSynapse(syn)
	}
}

func cmdGet(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: synapse ID required")
		os.Exit(1)
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid ID: %s\n", args[0])
		os.Exit(1)
	}

	store := getStore()
	syn, err := store.Get(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		jsonOut(syn)
		return
	}

	printSynapseDetailed(syn)
}

func cmdClaim(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: synapse ID required")
		os.Exit(1)
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid ID: %s\n", args[0])
		os.Exit(1)
	}

	store := getStore()
	syn, err := store.Get(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	syn.MarkInProgress()
	saveStore(store)

	if jsonOutput {
		jsonOut(syn)
		return
	}

	fmt.Printf("Claimed synapse #%d: %s\n", syn.ID, syn.Title)
	fmt.Printf("Status: %s\n", syn.Status)
}

func cmdDone(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: synapse ID required")
		os.Exit(1)
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid ID: %s\n", args[0])
		os.Exit(1)
	}

	store := getStore()
	syn, err := store.Get(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	syn.MarkDone()
	saveStore(store)

	if jsonOutput {
		jsonOut(syn)
		return
	}

	fmt.Printf("Completed synapse #%d: %s\n", syn.ID, syn.Title)
}

func printSynapse(syn *types.Synapse) {
	statusIcon := statusToIcon(syn.Status)
	fmt.Printf("%s [%s] #%d: %s\n", statusIcon, syn.Status, syn.ID, syn.Title)
	if syn.Assignee != "" {
		fmt.Printf("   Assignee: %s\n", syn.Assignee)
	}
	if len(syn.BlockedBy) > 0 {
		fmt.Printf("   Blocked by: %v\n", syn.BlockedBy)
	}
	fmt.Println()
}

func printSynapseDetailed(syn *types.Synapse) {
	fmt.Printf("Synapse #%d\n", syn.ID)
	fmt.Printf("  Title:       %s\n", syn.Title)
	fmt.Printf("  Status:      %s %s\n", statusToIcon(syn.Status), syn.Status)
	if syn.Description != "" {
		fmt.Printf("  Description: %s\n", syn.Description)
	}
	if syn.Assignee != "" {
		fmt.Printf("  Assignee:    %s\n", syn.Assignee)
	}
	if syn.ParentID > 0 {
		fmt.Printf("  Parent:      #%d\n", syn.ParentID)
	}
	if len(syn.BlockedBy) > 0 {
		fmt.Printf("  Blocked by:  %v\n", syn.BlockedBy)
	}
	fmt.Printf("  Created:     %s\n", syn.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Updated:     %s\n", syn.UpdatedAt.Format("2006-01-02 15:04:05"))
}

func statusToIcon(status types.Status) string {
	switch status {
	case types.StatusOpen:
		return "○"
	case types.StatusInProgress:
		return "◐"
	case types.StatusBlocked:
		return "◌"
	case types.StatusReview:
		return "◑"
	case types.StatusDone:
		return "●"
	default:
		return "?"
	}
}

func cmdDoneAll() {
	store := getStore()
	all := store.All()

	count := 0
	for _, syn := range all {
		if syn.Status != types.StatusDone {
			syn.MarkDone()
			count++
		}
	}

	if count == 0 {
		if jsonOutput {
			jsonOut(map[string]int{"count": 0})
			return
		}
		fmt.Println("No tasks to mark as done")
		return
	}

	saveStore(store)

	if jsonOutput {
		jsonOut(map[string]int{"count": count})
		return
	}

	fmt.Printf("Marked %d task(s) as done\n", count)
}

func cmdDelete(args []string) {
	store := getStore()

	// Check for --all flag
	if len(args) > 0 && args[0] == "--all" {
		all := store.All()
		count := len(all)
		if count == 0 {
			if jsonOutput {
				jsonOut(map[string]int{"deleted": 0})
				return
			}
			fmt.Println("No tasks to delete")
			return
		}

		if err := store.DeleteAll(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		saveStore(store)

		if jsonOutput {
			jsonOut(map[string]int{"deleted": count})
			return
		}
		fmt.Printf("Deleted all %d task(s)\n", count)
		return
	}

	// Check for --done flag (cleanup completed tasks)
	if len(args) > 0 && args[0] == "--done" {
		count, err := store.DeleteByStatus(types.StatusDone)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if count == 0 {
			if jsonOutput {
				jsonOut(map[string]int{"deleted": 0})
				return
			}
			fmt.Println("No completed tasks to delete")
			return
		}

		saveStore(store)

		if jsonOutput {
			jsonOut(map[string]int{"deleted": count})
			return
		}
		fmt.Printf("Deleted %d completed task(s)\n", count)
		return
	}

	// Delete single task by ID
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: synapse ID required (or use --all/--done to delete tasks)")
		os.Exit(1)
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid ID: %s\n", args[0])
		os.Exit(1)
	}

	syn, err := store.Get(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Snapshot for JSON output before deletion
	snapshot := *syn
	if err := store.Delete(id); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	saveStore(store)

	if jsonOutput {
		jsonOut(&snapshot)
		return
	}
	fmt.Printf("Deleted synapse #%d: %s\n", id, snapshot.Title)
}

func getBreadcrumbStore() *storage.BreadcrumbStore {
	store := storage.NewBreadcrumbStore(storage.DefaultDir)
	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "error loading breadcrumbs: %v\n", err)
		os.Exit(1)
	}
	return store
}

func saveBreadcrumbStore(store *storage.BreadcrumbStore) {
	if err := store.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "error saving breadcrumbs: %v\n", err)
		os.Exit(1)
	}
}

func cmdBreadcrumb(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: subcommand required (set, get, list, delete)")
		os.Exit(1)
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "set":
		cmdBreadcrumbSet(subargs)
	case "get":
		cmdBreadcrumbGet(subargs)
	case "list", "ls":
		cmdBreadcrumbList(subargs)
	case "delete", "rm":
		cmdBreadcrumbDelete(subargs)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown breadcrumb subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func cmdBreadcrumbSet(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "error: key and value required")
		fmt.Fprintln(os.Stderr, "usage: synapse breadcrumb set <key> <value> [--task-id N]")
		os.Exit(1)
	}

	key := args[0]
	var value string
	var taskID int

	// Parse remaining arguments
	i := 1
	for i < len(args) {
		arg := args[i]
		if arg == "--task-id" && i+1 < len(args) {
			i++
			id, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid task ID: %s\n", args[i])
				os.Exit(1)
			}
			taskID = id
		} else if !strings.HasPrefix(arg, "--") {
			if value == "" {
				value = arg
			} else {
				value = value + " " + arg
			}
		} else {
			fmt.Fprintf(os.Stderr, "error: unknown flag: %s\n", arg)
			os.Exit(1)
		}
		i++
	}

	if value == "" {
		fmt.Fprintln(os.Stderr, "error: value required")
		os.Exit(1)
	}

	store := getBreadcrumbStore()
	_, err := store.Set(key, value, taskID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	saveBreadcrumbStore(store)

	if jsonOutput {
		b, _ := store.Get(key)
		jsonOut(b)
		return
	}

	fmt.Printf("Set breadcrumb: %s = %s\n", key, value)
	if taskID > 0 {
		fmt.Printf("  Linked to task #%d\n", taskID)
	}
}

func cmdBreadcrumbGet(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: key required")
		fmt.Fprintln(os.Stderr, "usage: synapse breadcrumb get <key>")
		os.Exit(1)
	}

	key := args[0]
	store := getBreadcrumbStore()

	b, found := store.Get(key)
	if !found {
		fmt.Fprintf(os.Stderr, "breadcrumb not found: %s\n", key)
		os.Exit(1)
	}

	if jsonOutput {
		jsonOut(b)
		return
	}

	// Output just the value for easy scripting
	fmt.Println(b.Value)
}

func cmdBreadcrumbList(args []string) {
	var prefix string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			prefix = arg
		}
	}

	store := getBreadcrumbStore()
	breadcrumbs := store.List(prefix)

	if jsonOutput {
		jsonOut(breadcrumbs)
		return
	}

	if len(breadcrumbs) == 0 {
		if prefix != "" {
			fmt.Printf("No breadcrumbs found with prefix: %s\n", prefix)
		} else {
			fmt.Println("No breadcrumbs found")
		}
		return
	}

	fmt.Printf("Breadcrumbs (%d):\n\n", len(breadcrumbs))
	for _, b := range breadcrumbs {
		// Truncate long values for display
		value := b.Value
		if len(value) > 50 {
			value = value[:47] + "..."
		}
		fmt.Printf("  %s = %s\n", b.Key, value)
		if b.TaskID > 0 {
			fmt.Printf("    Task: #%d\n", b.TaskID)
		}
	}
}

func cmdBreadcrumbDelete(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: key required")
		fmt.Fprintln(os.Stderr, "usage: synapse breadcrumb delete <key>")
		os.Exit(1)
	}

	key := args[0]
	store := getBreadcrumbStore()

	if !store.Delete(key) {
		fmt.Fprintf(os.Stderr, "breadcrumb not found: %s\n", key)
		os.Exit(1)
	}

	saveBreadcrumbStore(store)

	if jsonOutput {
		jsonOut(map[string]string{"deleted": key})
		return
	}
	fmt.Printf("Deleted breadcrumb: %s\n", key)
}

func cmdSkill(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: subcommand required (install, uninstall, list, update, show)")
		os.Exit(1)
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "install":
		cmdSkillInstall(subargs)
	case "uninstall":
		cmdSkillUninstall(subargs)
	case "list", "ls":
		cmdSkillList()
	case "update":
		cmdSkillUpdate(subargs)
	case "show":
		cmdSkillShow()
	default:
		fmt.Fprintf(os.Stderr, "error: unknown skill subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func cmdSkillInstall(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: agent name required")
		fmt.Fprintf(os.Stderr, "available agents: %s\n", strings.Join(skill.AgentNames(), ", "))
		os.Exit(1)
	}

	agentName := args[0]
	level := skill.LevelProject

	for i := 1; i < len(args); i++ {
		if args[i] == "--level" && i+1 < len(args) {
			i++
			switch args[i] {
			case "user":
				level = skill.LevelUser
			case "project":
				level = skill.LevelProject
			default:
				fmt.Fprintf(os.Stderr, "error: invalid level: %s (must be 'user' or 'project')\n", args[i])
				os.Exit(1)
			}
		}
	}

	if err := skill.Install(agentName, level, version); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cfg, _ := skill.GetAgent(agentName)
	target := skill.TargetPath(cfg, level)
	fmt.Printf("Installed synapse skill for %s (%s)\n", cfg.DisplayName, level)
	fmt.Printf("  Path: %s\n", target)
}

func cmdSkillUninstall(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: agent name required")
		fmt.Fprintf(os.Stderr, "available agents: %s\n", strings.Join(skill.AgentNames(), ", "))
		os.Exit(1)
	}

	agentName := args[0]
	level := skill.LevelProject

	for i := 1; i < len(args); i++ {
		if args[i] == "--level" && i+1 < len(args) {
			i++
			switch args[i] {
			case "user":
				level = skill.LevelUser
			case "project":
				level = skill.LevelProject
			default:
				fmt.Fprintf(os.Stderr, "error: invalid level: %s (must be 'user' or 'project')\n", args[i])
				os.Exit(1)
			}
		}
	}

	if err := skill.Uninstall(agentName, level); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cfg, _ := skill.GetAgent(agentName)
	fmt.Printf("Uninstalled synapse skill for %s (%s)\n", cfg.DisplayName, level)
}

func cmdSkillList() {
	infos := skill.List()

	fmt.Println("Synapse Skill Installations:")
	fmt.Println()

	lastAgent := ""
	for _, info := range infos {
		if info.Agent != lastAgent {
			cfg, _ := skill.GetAgent(info.Agent)
			fmt.Printf("  %s (%s):\n", cfg.DisplayName, info.Agent)
			lastAgent = info.Agent
		}

		status := "not installed"
		if info.Installed {
			if info.Version != "" {
				status = fmt.Sprintf("v%s", info.Version)
			} else {
				status = "installed"
			}
		}

		fmt.Printf("    %-8s %s\n", info.Level+":", status)
	}
}

func cmdSkillUpdate(args []string) {
	level := skill.LevelProject
	var agentName string

	for i := 0; i < len(args); i++ {
		if args[i] == "--level" && i+1 < len(args) {
			i++
			switch args[i] {
			case "user":
				level = skill.LevelUser
			case "project":
				level = skill.LevelProject
			default:
				fmt.Fprintf(os.Stderr, "error: invalid level: %s (must be 'user' or 'project')\n", args[i])
				os.Exit(1)
			}
		} else if !strings.HasPrefix(args[i], "--") {
			agentName = args[i]
		}
	}

	if agentName != "" {
		// Update specific agent
		if !skill.IsInstalled(agentName, level) {
			fmt.Fprintf(os.Stderr, "error: %s is not installed at %s level\n", agentName, level)
			os.Exit(1)
		}
		if err := skill.Update(agentName, level, version); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		cfg, _ := skill.GetAgent(agentName)
		fmt.Printf("Updated synapse skill for %s (%s) to v%s\n", cfg.DisplayName, level, version)
	} else {
		// Update all installed
		updated, err := skill.UpdateAll(version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(updated) == 0 {
			fmt.Println("No installed skills to update")
			return
		}
		fmt.Printf("Updated %d installation(s) to v%s:\n", len(updated), version)
		for _, name := range updated {
			fmt.Printf("  %s\n", name)
		}
	}
}

func cmdSkillShow() {
	content, err := skill.ShowSkillContent(version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(content)
}

func cmdServe() {
	store := getStore()
	bcStore := getBreadcrumbStore()
	server := mcp.NewServer(store, bcStore)
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func cmdView(args []string) {
	port := 8080

	for i := 0; i < len(args); i++ {
		if args[i] == "--port" && i+1 < len(args) {
			i++
			p, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid port: %s\n", args[i])
				os.Exit(1)
			}
			port = p
		}
	}

	store := getStore()
	server := view.NewServer(store, port)
	fmt.Printf("Starting visualization at http://localhost:%d\n", port)
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
