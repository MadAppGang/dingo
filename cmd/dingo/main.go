// Package main implements the Dingo compiler CLI
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"github.com/MadAppGang/dingo/pkg/ui"
	"github.com/MadAppGang/dingo/pkg/ui/mascot"
)

var (
	version = "0.6.0"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dingo",
		Short: "Dingo - A meta-language for Go",
		Long: `Dingo is a meta-language that transpiles to idiomatic Go code.
It provides Result/Option types, pattern matching, error propagation,
and other quality-of-life features while maintaining 100% Go ecosystem compatibility.`,
		Version: version,
		SilenceUsage:  true, // Don't show usage on errors
		SilenceErrors: true, // We handle error display ourselves
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintDingoHelp(version)
		},
	}

	// Store default help function before overriding
	defaultHelpFunc := rootCmd.HelpFunc()

	// Override help flag to use our custom help ONLY for root command
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Only use custom help for root command, not subcommands
		if cmd == rootCmd {
			ui.PrintDingoHelp(version)
		} else {
			// Use default help for subcommands
			defaultHelpFunc(cmd, args)
		}
	})

	rootCmd.AddCommand(goBuildCmd())  // dingo build - transpile + go build
	rootCmd.AddCommand(goRunCmd())    // dingo run - transpile + go run
	rootCmd.AddCommand(goCmd())       // dingo go - transpile only
	rootCmd.AddCommand(lintCmd())     // dingo lint - run linter
	rootCmd.AddCommand(fmtCmd())      // dingo fmt - format files
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(mascotCmd())

	if err := rootCmd.Execute(); err != nil {
		// Error is already printed by cobra
		os.Exit(1)
	}
}

// Global flags
var simulateSlow bool
var noMascot bool

// goCmd is the "dingo go" command - transpile only (.dingo → .go)
func goCmd() *cobra.Command {
	var (
		output string
		outdir string
		watch  bool
	)

	cmd := &cobra.Command{
		Use:   "go [file.dingo | ./...]",
		Short: "Transpile Dingo source files to Go (no compilation)",
		Long: `The 'go' command transpiles Dingo source files (.dingo) to Go source files (.go).

This is useful for:
- Inspecting generated Go code
- Using with external Go tooling
- IDE integration

The transpiler:
1. Parses Dingo source code into AST
2. Transforms Dingo-specific features to Go equivalents
3. Generates idiomatic Go code

Examples:
  dingo go hello.dingo              # Generates hello.go in same directory
  dingo go -o output.go main.dingo  # Custom output file
  dingo go ./...                    # Transpile all .dingo files in workspace`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTranspile(args, output, outdir, watch)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (single file only)")
	cmd.Flags().StringVarP(&outdir, "outdir", "O", "", "Output directory (mirrors source structure)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for file changes and rebuild")
	cmd.Flags().BoolVar(&simulateSlow, "slow", false, "Simulate slow build (3s delay) for testing animation")
	cmd.Flags().BoolVar(&noMascot, "no-mascot", false, "Disable mascot and animation (plain text output)")

	return cmd
}

// goRunCmd is defined in compile.go - it's like goBuildCmd but invokes go run

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Dingo",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintVersionInfo(version)
		},
	}
}

func runTranspile(files []string, output, outdir string, watch bool) error {
	// Validate mutually exclusive flags
	if output != "" && outdir != "" {
		return fmt.Errorf("--output and --outdir are mutually exclusive")
	}

	// Load config (for future use)
	cfg, err := config.Load(nil)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	_ = cfg // Reserved for future config options

	// Expand workspace patterns (./..., ./pkg/...) to actual files
	expandedFiles, err := expandWorkspacePatterns(files)
	if err != nil {
		return fmt.Errorf("failed to expand workspace patterns: %w", err)
	}

	// Output directory not yet supported
	if outdir != "" {
		return fmt.Errorf("--outdir flag is not currently supported")
	}

	// Create build UI (always - mascot shown at end)
	buildUI := ui.NewSimpleBuildUI()
	buildUI.Start()
	defer buildUI.Stop()

	// Print header
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	fmt.Printf("%s %s\n\n", titleStyle.Render("🐕 Dingo"), versionStyle.Render("v"+version))

	// Print build start
	if len(expandedFiles) == 1 {
		fmt.Println("Building 1 file")
	} else {
		fmt.Printf("Building %d files\n", len(expandedFiles))
	}

	// Build each file
	success := true
	var lastError error
	transpiled := 0

	for _, file := range expandedFiles {
		var outputPath string

		if output != "" {
			// Using single output file mode
			outputPath = output
		}

		if err := buildFileSimple(file, outputPath, buildUI); err != nil {
			success = false
			lastError = err
			buildUI.SetStatus(mascot.StateFailed, "Build failed!", "see error above")
			fmt.Printf("  ✗ %s\n", err.Error())
			break
		}
		transpiled++
	}

	// Final status
	if success {
		buildUI.SetStatus(mascot.StateSuccess, "Build successful!", fmt.Sprintf("%d file(s) transpiled", transpiled))
		if watch {
			fmt.Println("\nℹ Watch mode not yet implemented")
		}
	}

	return lastError
}

func buildFile(inputPath, outputPath string, buildUI *ui.BuildOutput) error {
	if outputPath == "" {
		// Default: replace .dingo with .go
		if len(inputPath) > 6 && inputPath[len(inputPath)-6:] == ".dingo" {
			outputPath = inputPath[:len(inputPath)-6] + ".go"
		} else {
			outputPath = inputPath + ".go"
		}
	}

	// Print file header
	buildUI.PrintFileStart(inputPath, outputPath)

	// Step 1: Read source
	readStart := time.Now()
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	buildUI.PrintStep(ui.Step{
		Name:     "Read",
		Status:   ui.StepSuccess,
		Duration: time.Since(readStart),
	})

	// Step 2: Transpile using pure AST pipeline with mappings
	transpileStart := time.Now()
	transpileResult, err := transpiler.PureASTTranspileWithMappings(src, inputPath, true)
	if err != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Transpile",
			Status:   ui.StepError,
			Duration: time.Since(transpileStart),
		})
		return fmt.Errorf("transpilation error: %w", err)
	}
	goSource := transpileResult.GoCode

	buildUI.PrintStep(ui.Step{
		Name:     "Transpile",
		Status:   ui.StepSuccess,
		Duration: time.Since(transpileStart),
	})

	// Step 3: Write output files
	writeStart := time.Now()

	// Write .go file
	if err := os.WriteFile(outputPath, goSource, 0o644); err != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Write",
			Status:   ui.StepError,
			Duration: time.Since(writeStart),
		})
		return fmt.Errorf("failed to write output: %w", err)
	}

	// Write .dmap file to project root .dmap/ folder
	dmapPath, dmapErr := calculateDmapPath(inputPath)
	if dmapErr != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Write",
			Status:   ui.StepSuccess,
			Duration: time.Since(writeStart),
			Message:  fmt.Sprintf("%d bytes written (source map warning: %v)", len(goSource), dmapErr),
		})
		return nil
	}

	writer := dmap.NewWriter(transpileResult.DingoSource, transpileResult.GoCode)
	if dmapErr := writer.WriteFile(dmapPath, transpileResult.LineMappings); dmapErr != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Write",
			Status:   ui.StepSuccess,
			Duration: time.Since(writeStart),
			Message:  fmt.Sprintf("%d bytes written (source map warning: %v)", len(goSource), dmapErr),
		})
	} else {
		buildUI.PrintStep(ui.Step{
			Name:     "Write",
			Status:   ui.StepSuccess,
			Duration: time.Since(writeStart),
			Message:  fmt.Sprintf("%d bytes written", len(goSource)),
		})
	}

	return nil
}

// buildFileSimple builds a single file with animated spinner during steps
func buildFileSimple(inputPath, outputPath string, buildUI *ui.SimpleBuildUI) error {
	if outputPath == "" {
		// Default: replace .dingo with .go
		if len(inputPath) > 6 && inputPath[len(inputPath)-6:] == ".dingo" {
			outputPath = inputPath[:len(inputPath)-6] + ".go"
		} else {
			outputPath = inputPath + ".go"
		}
	}

	// Get just the filename for display
	fileName := filepath.Base(inputPath)

	// File header
	inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4"))
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))
	fmt.Printf("  %s %s %s\n\n",
		inputStyle.Render(inputPath),
		arrowStyle.Render("→"),
		outputStyle.Render(outputPath))

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)

	// Step 1: Read source (with spinner)
	var src []byte
	var readDuration time.Duration
	err := runWithSpinner("Read", func() error {
		start := time.Now()
		var err error
		src, err = os.ReadFile(inputPath)
		readDuration = time.Since(start)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	fmt.Printf("  %s Read        Done %s\n",
		successStyle.Render("✓"),
		timeStyle.Render("("+formatDuration(readDuration)+")"))

	// Step 2: Transpile (with spinner - this is the long one)
	var goSource []byte
	var transpileResult transpiler.TranspileResult
	var transpileDuration time.Duration
	buildUI.SetStatus(mascot.StateCompiling, "Transpiling...", fileName)
	err = runWithSpinner("Transpile", func() error {
		start := time.Now()
		var err error
		transpileResult, err = transpiler.PureASTTranspileWithMappings(src, inputPath, true)
		goSource = transpileResult.GoCode
		// Simulate slow build if --slow flag is set
		if simulateSlow {
			time.Sleep(3 * time.Second)
		}
		transpileDuration = time.Since(start)
		return err
	})
	if err != nil {
		return fmt.Errorf("transpilation error: %w", err)
	}
	fmt.Printf("  %s Transpile   Done %s\n",
		successStyle.Render("✓"),
		timeStyle.Render("("+formatDuration(transpileDuration)+")"))

	// Step 3: Write output files (with spinner)
	var writeDuration time.Duration
	err = runWithSpinner("Write", func() error {
		start := time.Now()

		// Write .go file
		err := os.WriteFile(outputPath, goSource, 0o644)
		if err != nil {
			return err
		}

		// Write .dmap file to project root .dmap/ folder
		// This keeps source directories clean from generated files
		// Path: examples/03_option/user.dingo -> .dmap/examples/03_option/user.dmap
		dmapPath, dmapErr := calculateDmapPath(inputPath)
		if dmapErr != nil {
			fmt.Printf("\n  ⚠ Warning: Failed to calculate source map path: %v\n", dmapErr)
			writeDuration = time.Since(start)
			return nil
		}

		writer := dmap.NewWriter(transpileResult.DingoSource, transpileResult.GoCode)
		if dmapErr := writer.WriteFile(dmapPath, transpileResult.LineMappings); dmapErr != nil {
			// .dmap write failure is non-fatal - warn but don't fail build
			// This ensures build still works even if source mapping fails
			fmt.Printf("\n  ⚠ Warning: Failed to write source map: %v\n", dmapErr)
		}

		writeDuration = time.Since(start)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	fmt.Printf("  %s Write       Done %s\n",
		successStyle.Render("✓"),
		timeStyle.Render("("+formatDuration(writeDuration)+")"))
	fmt.Printf("    %s\n\n", timeStyle.Render(fmt.Sprintf("%d bytes written", len(goSource))))

	return nil
}

// runWithSpinner runs a function while showing animated mascot
func runWithSpinner(stepName string, fn func() error) error {
	// No animation in plain mode
	if noMascot {
		return fn()
	}

	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	frameIdx := 0
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	// Mascot frames with spinner eyes
	mascotFrames := [][]string{
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◜   ◜  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◝   ◝  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◞   ◞  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◟   ◟  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▄▀ ▀▄▄█▀      ",
		},
	}

	mascotColor := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	statusStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#45475A"))

	// Status text to show next to mascot
	statusLines := []string{
		"",
		statusStyle.Render(stepName + "..."),
		detailStyle.Render("please wait"),
	}

	// Draw initial mascot
	drawMascotFrame := func(frame []string) {
		// Separator
		fmt.Println(separatorStyle.Render("────────────────────────────────────────────────────────────"))

		maxLines := len(frame)
		for i := 0; i < maxLines; i++ {
			mascotLine := mascotColor.Render(frame[i])
			statusLine := ""
			if i < len(statusLines) {
				statusLine = "  " + statusLines[i]
			}
			fmt.Printf("%s%s\n", mascotLine, statusLine)
		}
	}

	// Hide cursor
	fmt.Print("\033[?25l")

	// Draw first frame
	drawMascotFrame(mascotFrames[frameIdx])
	mascotHeight := len(mascotFrames[0]) + 1 // +1 for separator

	for {
		select {
		case err := <-done:
			// Move cursor up and clear mascot area
			fmt.Printf("\033[%dA", mascotHeight)
			for i := 0; i < mascotHeight; i++ {
				fmt.Print("\033[2K\n")
			}
			fmt.Printf("\033[%dA", mascotHeight)
			// Show cursor
			fmt.Print("\033[?25h")
			return err
		case <-ticker.C:
			frameIdx = (frameIdx + 1) % len(mascotFrames)
			// Move cursor up to redraw mascot
			fmt.Printf("\033[%dA", mascotHeight)
			drawMascotFrame(mascotFrames[frameIdx])
		}
	}
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// calculateDmapPath calculates the .dmap file path in the project root .dmap/ folder.
// This keeps generated source maps separate from source files.
// Example: examples/03_option/user.dingo -> .dmap/examples/03_option/user.dmap
func calculateDmapPath(inputPath string) (string, error) {
	// Convert to absolute path
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Find workspace root (go.mod, go.work, or dingo.toml)
	inputDir := filepath.Dir(absInput)
	workspaceRoot, err := DetectWorkspaceRoot(inputDir)
	if err != nil {
		// Fall back to current directory if no workspace marker found
		workspaceRoot, _ = os.Getwd()
	}

	// Calculate relative path from workspace root
	relPath, err := filepath.Rel(workspaceRoot, absInput)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Replace .dingo extension with .dmap
	relDmap := strings.TrimSuffix(relPath, ".dingo") + ".dmap"

	// Build final path: workspaceRoot/.dmap/relPath
	dmapPath := filepath.Join(workspaceRoot, ".dmap", relDmap)

	// Ensure parent directory exists
	dmapDir := filepath.Dir(dmapPath)
	if err := os.MkdirAll(dmapDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create .dmap directory: %w", err)
	}

	return dmapPath, nil
}

// expandWorkspacePatterns expands workspace patterns like ./... to actual .dingo files
func expandWorkspacePatterns(patterns []string) ([]string, error) {
	var result []string

	for _, pattern := range patterns {
		// Check if this is a workspace pattern
		if isWorkspacePattern(pattern) {
			files, err := expandPattern(pattern)
			if err != nil {
				return nil, err
			}
			result = append(result, files...)
		} else {
			// Regular file path, keep as-is
			result = append(result, pattern)
		}
	}

	return result, nil
}

// isWorkspacePattern checks if a string is a workspace pattern (contains ...)
func isWorkspacePattern(s string) bool {
	return s == "..." || s == "./..." || strings.HasSuffix(s, "/...")
}

// expandPattern expands a workspace pattern to actual .dingo files
func expandPattern(pattern string) ([]string, error) {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Find workspace root
	// TODO: Re-enable workspace detection
	// root, err := DetectWorkspaceRoot(cwd)
	// if err != nil {
	// 	// Fall back to current directory if no workspace root found
	// 	root = cwd
	// }
	_ = cwd // Suppress unused variable warning

	// Scan workspace for packages
	// TODO: Re-enable workspace scanning
	// ws, err := ScanWorkspace(root)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to scan workspace: %w", err)
	// }

	// Collect files matching the pattern
	var files []string
	// TODO: Re-enable pattern matching
	// for _, pkg := range ws.Packages {
	// 	if MatchesPattern(pkg.Path, pattern) {
	// 		for _, dingoFile := range pkg.DingoFiles {
	// 			// Convert to absolute path
	// 			absPath := filepath.Join(root, dingoFile)
	// 			files = append(files, absPath)
	// 		}
	// 	}
	// }

	if len(files) == 0 {
		return nil, fmt.Errorf("no .dingo files found matching pattern: %s", pattern)
	}

	return files, nil
}

// mascotCmd creates the mascot debug command
func mascotCmd() *cobra.Command {
	var (
		state     string
		animate   bool
		duration  int
		listAll   bool
	)

	cmd := &cobra.Command{
		Use:   "mascot [state]",
		Short: "Debug command to test mascot animations and states",
		Long: `Display the Dingo mascot in various states for debugging.

Available states:
  idle       - Default idle state
  compiling  - Compiling/loading animation
  running    - Running state
  success    - Success celebration
  failed     - Error/failed state
  thinking   - Thinking animation
  help       - Friendly help pose

Examples:
  dingo mascot                    # Show default mascot
  dingo mascot --state success    # Show success state
  dingo mascot --state compiling --animate --duration 5
  dingo mascot --list             # List all available states`,
		Run: func(cmd *cobra.Command, args []string) {
			if listAll {
				listMascotStates()
				return
			}

			// Parse state from args or flag
			stateStr := state
			if len(args) > 0 {
				stateStr = args[0]
			}
			if stateStr == "" {
				stateStr = "idle"
			}

			runMascotDebug(stateStr, animate, duration)
		},
	}

	cmd.Flags().StringVarP(&state, "state", "s", "", "Mascot state to display")
	cmd.Flags().BoolVarP(&animate, "animate", "a", false, "Show animation (if available for state)")
	cmd.Flags().IntVarP(&duration, "duration", "d", 3, "Animation duration in seconds")
	cmd.Flags().BoolVarP(&listAll, "list", "l", false, "List all available states")

	return cmd
}

// listMascotStates prints available mascot states
func listMascotStates() {
	fmt.Println("Available mascot states:")
	fmt.Println()
	states := []struct {
		name string
		desc string
	}{
		{"idle", "Default idle state with occasional blink"},
		{"compiling", "Compiling/loading with spinner animation"},
		{"running", "Running state"},
		{"success", "Success celebration with happy face"},
		{"failed", "Error/failed state with sad face"},
		{"thinking", "Thinking animation, looking around"},
		{"help", "Friendly help pose"},
	}
	for _, s := range states {
		fmt.Printf("  %-12s %s\n", s.name, s.desc)
	}
	fmt.Println()
	fmt.Println("Use: dingo mascot <state> [--animate] [--duration N]")
}

// runMascotDebug displays the mascot in the specified state
func runMascotDebug(stateStr string, animate bool, durationSec int) {
	// Force color output for debug command (even in non-TTY)
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Map string to MascotState
	stateMap := map[string]mascot.MascotState{
		"idle":      mascot.StateIdle,
		"compiling": mascot.StateCompiling,
		"running":   mascot.StateRunning,
		"success":   mascot.StateSuccess,
		"failed":    mascot.StateFailed,
		"thinking":  mascot.StateThinking,
		"help":      mascot.StateHelp,
	}

	// Map state to color scheme
	colorMap := map[string]mascot.ColorScheme{
		"idle":      mascot.DefaultColorScheme,
		"compiling": mascot.CompileColorScheme,
		"running":   mascot.DefaultColorScheme,
		"success":   mascot.SuccessColorScheme,
		"failed":    mascot.FailureColorScheme,
		"thinking":  mascot.DefaultColorScheme,
		"help":      mascot.DefaultColorScheme,
	}

	state, ok := stateMap[stateStr]
	if !ok {
		fmt.Printf("Unknown state: %s\n", stateStr)
		fmt.Println("Use 'dingo mascot --list' to see available states")
		os.Exit(1)
	}

	colorScheme := colorMap[stateStr]

	fmt.Printf("Showing mascot state: %s\n", stateStr)
	if animate {
		fmt.Printf("Animation duration: %d seconds\n", durationSec)
	}
	fmt.Println()

	// Create mascot
	m := mascot.New(
		mascot.WithInitialState(state),
		mascot.WithColorScheme(colorScheme),
		mascot.WithWriter(os.Stdout),
	)

	if animate {
		// For animation, render frames with colors in a loop
		runColoredAnimation(m, colorScheme, durationSec)
	} else {
		// Show single static frame with colors
		frame := m.Render()
		for _, line := range frame {
			fmt.Println(colorScheme.ApplyBodyColor(line))
		}
	}
}

// runColoredAnimation runs an animation with colors applied to each frame
func runColoredAnimation(m *mascot.Mascot, colorScheme mascot.ColorScheme, durationSec int) {
	// Get animation frames based on state
	frames := mascot.GetAnimationFrames(m)
	if len(frames) == 0 {
		fmt.Println("No animation frames available")
		return
	}

	// Hide cursor during animation
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h") // Show cursor on exit

	endTime := time.Now().Add(time.Duration(durationSec) * time.Second)
	frameIndex := 0
	lastHeight := 0

	for time.Now().Before(endTime) {
		// Clear previous frame
		if lastHeight > 0 {
			// Move cursor up
			fmt.Printf("\033[%dA", lastHeight)
		}

		// Get current frame
		frame := frames[frameIndex]
		lastHeight = len(frame)

		// Print frame with colors
		for _, line := range frame {
			// Clear line and print
			fmt.Print("\033[2K") // Clear entire line
			fmt.Println(colorScheme.ApplyBodyColor(line))
		}

		// Advance to next frame (loop)
		frameIndex = (frameIndex + 1) % len(frames)

		time.Sleep(150 * time.Millisecond)
	}
}
