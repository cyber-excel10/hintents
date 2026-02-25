// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dotandev/hintents/internal/dwarf"
	"github.com/dotandev/hintents/internal/visualizer"
)

// InteractiveViewer provides a terminal-based interactive trace navigation interface
type InteractiveViewer struct {
	trace  *ExecutionTrace
	reader *bufio.Reader
	trap   *TrapInfo
	dwarfParser *dwarf.Parser
}

// NewInteractiveViewer creates a new interactive trace viewer
func NewInteractiveViewer(trace *ExecutionTrace) *InteractiveViewer {
	viewer := &InteractiveViewer{
		trace:  trace,
		reader: bufio.NewReader(os.Stdin),
	}

	// Detect any traps in the trace
	detector := &TrapDetector{}
	viewer.trap = detector.FindTrapPoint(trace)

	return viewer
}

// NewInteractiveViewerWithWASM creates a new interactive trace viewer with WASM data for local variable inspection
func NewInteractiveViewerWithWASM(trace *ExecutionTrace, wasmData []byte) *InteractiveViewer {
	viewer := &InteractiveViewer{
		trace:  trace,
		reader: bufio.NewReader(os.Stdin),
	}

	// Initialize DWARF parser if WASM data is provided
	if len(wasmData) > 0 {
		parser, err := dwarf.NewParser(wasmData)
		if err == nil && parser.HasDebugInfo() {
			viewer.dwarfParser = parser
		}
	}

	// Detect any traps in the trace
	detector := &TrapDetector{}
	viewer.trap = detector.FindTrapPoint(trace)

	return viewer
}

// Start begins the interactive trace viewing session
func (v *InteractiveViewer) Start() error {
	fmt.Printf("%s ERST Interactive Trace Viewer\n", visualizer.Symbol("magnify"))
	fmt.Println("=================================")
	fmt.Printf("Transaction: %s\n", v.trace.TransactionHash)
	fmt.Printf("Total Steps: %d\n\n", len(v.trace.States))

	// Show trap info at startup if detected
	if v.trap != nil {
		fmt.Printf("%s Memory Trap Detected!\n", visualizer.Symbol("warn"))
		fmt.Printf("Type: %s\n", v.trap.Type)
		if v.trap.SourceLocation != nil {
			fmt.Printf("Location: %s:%d\n", v.trap.SourceLocation.File, v.trap.SourceLocation.Line)
		}
		fmt.Println("  Use 't' or 'trap' command to see local variables\n")
	}

	v.showHelp()
	v.displayCurrentState()

	for {
		fmt.Print("\n> ")
		input, err := v.reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		command := strings.TrimSpace(input)
		if command == "" {
			continue
		}

		if v.handleCommand(command) {
			break // Exit requested
		}
	}

	return nil
}

// handleCommand processes user commands and returns true if exit is requested
func (v *InteractiveViewer) handleCommand(command string) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}

	cmdExact := parts[0]
	cmd := strings.ToLower(cmdExact)

	// Handle case-sensitive 'S' for the stdlib toggle before the lowercased switch
	if cmdExact == "S" {
		v.hideStdLib = !v.hideStdLib
		status := "shown"
		if v.hideStdLib {
			status = "hidden"
		}
		fmt.Printf("%s Rust core::* traces are now %s\n", visualizer.Symbol("eye"), status)
		return false
	}

	switch cmd {
	case "n", "next", "forward":
		v.stepForward()
	case "p", "prev", "back", "backward":
		v.stepBackward()
	case "j", "jump":
		if len(parts) > 1 {
			v.jumpToStep(parts[1])
		} else {
			fmt.Println("Usage: jump <step_number>")
		}
	case "s", "show", "state":
		v.displayCurrentState()
	case "r", "reconstruct":
		if len(parts) > 1 {
			v.reconstructState(parts[1])
		} else {
			v.reconstructCurrentState()
		}
	case "t", "trap":
		v.displayTrapInfo()
	case "i", "info":
		v.showNavigationInfo()
	case "l", "list":
		if len(parts) > 1 {
			v.listSteps(parts[1])
		} else {
			v.listSteps("10")
		}
	case "h", "help":
		v.showHelp()
	case "q", "quit", "exit":
		fmt.Printf("Goodbye! %s\n", visualizer.Symbol("wave"))
		return true
	default:
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", cmdExact)
	}

	return false
}

// stepForward moves to the next step, skipping core::* if hideStdLib is true
func (v *InteractiveViewer) stepForward() {
	for {
		state, err := v.trace.StepForward()
		if err != nil {
			fmt.Printf("%s %s\n", visualizer.Error(), err)
			return
		}

		if v.hideStdLib && strings.HasPrefix(state.Function, "core::") {
			continue // Skip this step and automatically evaluate the next one
		}

		fmt.Printf("%s  Stepped forward to step %d\n", visualizer.Symbol("arrow_r"), state.Step)
		v.displayCurrentState()
		return
	}
}

// stepBackward moves to the previous step, skipping core::* if hideStdLib is true
func (v *InteractiveViewer) stepBackward() {
	for {
		state, err := v.trace.StepBackward()
		if err != nil {
			fmt.Printf("%s %s\n", visualizer.Error(), err)
			return
		}

		if v.hideStdLib && strings.HasPrefix(state.Function, "core::") {
			continue // Skip this step and automatically evaluate the previous one
		}

		fmt.Printf("%s  Stepped backward to step %d\n", visualizer.Symbol("arrow_l"), state.Step)
		v.displayCurrentState()
		return
	}
}

// jumpToStep jumps to a specific step
func (v *InteractiveViewer) jumpToStep(stepStr string) {
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		fmt.Printf("%s Invalid step number: %s\n", visualizer.Error(), stepStr)
		return
	}

	state, err := v.trace.JumpToStep(step)
	if err != nil {
		fmt.Printf("%s %s\n", visualizer.Error(), err)
		return
	}

	fmt.Printf("%s Jumped to step %d\n", visualizer.Symbol("target"), state.Step)
	v.displayCurrentState()
}

// displayCurrentState shows the current execution state
func (v *InteractiveViewer) displayCurrentState() {
	state, err := v.trace.GetCurrentState()
	if err != nil {
		fmt.Printf("%s %s\n", visualizer.Error(), err)
		return
	}

	fmt.Printf("\n%s Current State\n", visualizer.Symbol("pin"))
	fmt.Println("================")
	fmt.Printf("Step: %d/%d\n", state.Step, len(v.trace.States)-1)
	fmt.Printf("Time: %s\n", state.Timestamp.Format("15:04:05.000"))
	fmt.Printf("Operation: %s\n", state.Operation)

	if state.ContractID != "" {
		fmt.Printf("Contract: %s\n", state.ContractID)
	}
	if state.Function != "" {
		fmt.Printf("Function: %s\n", state.Function)
	}
	if len(state.Arguments) > 0 {
		fmt.Printf("Arguments: %v\n", state.Arguments)
	}
	if state.ReturnValue != nil {
		fmt.Printf("Return: %v\n", state.ReturnValue)
	}
	if state.Error != "" {
		fmt.Printf("%s Error: %s\n", visualizer.Error(), state.Error)
		// Suggest using trap command
		if v.trap != nil && IsMemoryTrap(v.trap) {
			fmt.Println("\n  Use 't' or 'trap' to see local variables at this point")
		}
	}

	// Show memory/state summary
	if len(state.HostState) > 0 {
		fmt.Printf("Host State: %d entries\n", len(state.HostState))
	}
	if len(state.Memory) > 0 {
		fmt.Printf("Memory: %d entries\n", len(state.Memory))
	}
}

// reconstructCurrentState reconstructs and displays the current state
func (v *InteractiveViewer) reconstructCurrentState() {
	state, err := v.trace.ReconstructStateAt(v.trace.CurrentStep)
	if err != nil {
		fmt.Printf("%s Failed to reconstruct state: %s\n", visualizer.Error(), err)
		return
	}

	fmt.Printf("\n%s Reconstructed State\n", visualizer.Symbol("wrench"))
	fmt.Println("======================")
	v.displayState(state)
}

// reconstructState reconstructs and displays state at a specific step
func (v *InteractiveViewer) reconstructState(stepStr string) {
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		fmt.Printf("%s Invalid step number: %s\n", visualizer.Error(), stepStr)
		return
	}

	state, err := v.trace.ReconstructStateAt(step)
	if err != nil {
		fmt.Printf("%s Failed to reconstruct state: %s\n", visualizer.Error(), err)
		return
	}

	fmt.Printf("\n%s Reconstructed State at Step %d\n", visualizer.Symbol("wrench"), step)
	fmt.Println("==================================")
	v.displayState(state)
}

// displayState displays a complete state
func (v *InteractiveViewer) displayState(state *ExecutionState) {
	fmt.Printf("Step: %d\n", state.Step)
	fmt.Printf("Time: %s\n", state.Timestamp.Format("15:04:05.000"))
	fmt.Printf("Operation: %s\n", state.Operation)

	if state.ContractID != "" {
		fmt.Printf("Contract: %s\n", state.ContractID)
	}
	if state.Function != "" {
		fmt.Printf("Function: %s\n", state.Function)
	}

	if len(state.HostState) > 0 {
		fmt.Println("\nHost State:")
		for k, val := range state.HostState {
			fmt.Printf("  %s: %v\n", k, val)
		}
	}

	if len(state.Memory) > 0 {
		fmt.Println("\nMemory:")
		for k, val := range state.Memory {
			fmt.Printf("  %s: %v\n", k, val)
		}
	}
}

// showNavigationInfo displays navigation information
func (v *InteractiveViewer) showNavigationInfo() {
	info := v.trace.GetNavigationInfo()

	fmt.Printf("\n%s Navigation Info\n", visualizer.Symbol("chart"))
	fmt.Println("==================")
	fmt.Printf("Total Steps: %d\n", info["total_steps"])
	fmt.Printf("Current Step: %d\n", info["current_step"])
	fmt.Printf("Can Step Back: %t\n", info["can_step_back"])
	fmt.Printf("Can Step Forward: %t\n", info["can_step_forward"])
	fmt.Printf("Snapshots: %d\n", info["snapshots_count"])

	// Show trap info if detected
	if v.trap != nil {
		fmt.Printf("\n%s Trap Detected: %s\n", visualizer.Error(), v.trap.Type)
		fmt.Println("  Type 't' or 'trap' to see details with local variables")
	}
}

// displayTrapInfo displays trap information including local variables
func (v *InteractiveViewer) displayTrapInfo() {
	if v.trap == nil {
		fmt.Printf("%s No trap detected in this trace\n", visualizer.Symbol("check"))
		return
	}

	fmt.Println("\n" + FormatTrapInfo(v.trap))
}

// listSteps shows a list of steps around the current position
func (v *InteractiveViewer) listSteps(countStr string) {
	count, err := strconv.Atoi(countStr)
	if err != nil {
		count = 10
	}

	current := v.trace.CurrentStep
	start := max(0, current-count/2)
	end := min(len(v.trace.States)-1, start+count-1)

	fmt.Printf("\n%s Steps %d-%d\n", visualizer.Symbol("list"), start, end)
	if v.hideStdLib {
		fmt.Println("(Filtering out core::* traces)")
	}
	fmt.Println("===============")

	for i := start; i <= end; i++ {
		state := &v.trace.States[i]

		// Hide core::* functions from the list view if toggle is active
		if v.hideStdLib && strings.HasPrefix(state.Function, "core::") {
			continue
		}

		marker := "  "
		if i == current {
			marker = visualizer.Symbol("play")
		}

		fmt.Printf("%s %3d: %s", marker, i, state.Operation)
		if state.Function != "" {
			fmt.Printf(" (%s)", state.Function)
		}
		if state.Error != "" {
			fmt.Printf(" %s", visualizer.Error())
		}
		fmt.Println()
	}
}

// showHelp displays available commands
func (v *InteractiveViewer) showHelp() {
	fmt.Printf("\n%s Available Commands\n", visualizer.Symbol("book"))
	fmt.Println("=====================")
	fmt.Println("Navigation:")
	fmt.Println("  n, next, forward     - Step forward")
	fmt.Println("  p, prev, back        - Step backward")
	fmt.Println("  j, jump <step>       - Jump to specific step")
	fmt.Println()
	fmt.Println("Display:")
	fmt.Println("  s, show, state       - Show current state")
	fmt.Println("  S                    - Toggle hiding/showing Rust core::* traces")
	fmt.Println("  r, reconstruct [step] - Reconstruct state")
	fmt.Println("  t, trap              - Show trap info with local variables")
	fmt.Println("  l, list [count]      - List steps (default: 10)")
	fmt.Println("  i, info              - Show navigation info")
	fmt.Println()
	fmt.Println("Other:")
	fmt.Println("  h, help              - Show this help")
	fmt.Println("  q, quit, exit        - Exit viewer")
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
