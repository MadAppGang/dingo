// Package main implements the Dingo compiler CLI
package main

import (
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/dingo/pkg/generator"
	"github.com/yourusername/dingo/pkg/parser"
	"github.com/yourusername/dingo/pkg/ui"
)

var (
	version = "0.1.0-alpha"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dingo",
		Short: "Dingo - A meta-language for Go",
		Long: `Dingo is a meta-language that transpiles to idiomatic Go code.
It provides Result/Option types, pattern matching, error propagation,
and other quality-of-life features while maintaining 100% Go ecosystem compatibility.`,
		Version: version,
		SilenceUsage: true, // Don't show usage on errors
	}

	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		// Error is already printed by cobra
		os.Exit(1)
	}
}

func buildCmd() *cobra.Command {
	var (
		output string
		watch  bool
	)

	cmd := &cobra.Command{
		Use:   "build [file.dingo]",
		Short: "Transpile Dingo source files to Go",
		Long: `Build command transpiles Dingo source files (.dingo) to Go source files (.go).

The transpiler:
1. Parses Dingo source code into AST
2. Transforms Dingo-specific features to Go equivalents
3. Generates idiomatic Go code with source maps

Example:
  dingo build hello.dingo          # Generates hello.go
  dingo build -o output.go main.dingo
  dingo build *.dingo              # Build all .dingo files`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(args, output, watch)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (default: replace .dingo with .go)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for file changes and rebuild")

	return cmd
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [file.dingo] [-- args...]",
		Short: "Compile and run a Dingo program",
		Long: `Run compiles a Dingo source file and executes it immediately.

This is equivalent to:
  dingo build file.dingo
  go run file.go

The generated .go file is created and then executed. You can pass arguments
to your program after -- (double dash).

Examples:
  dingo run hello.dingo
  dingo run main.dingo -- arg1 arg2 arg3
  dingo run server.dingo -- --port 8080`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputFile := args[0]
			programArgs := []string{}

			// If there are args after --, pass them to the program
			if len(args) > 1 {
				programArgs = args[1:]
			}

			return runDingoFile(inputFile, programArgs)
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Dingo",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintVersionInfo(version)
		},
	}
}

func runBuild(files []string, output string, watch bool) error {
	// Create beautiful output handler
	buildUI := ui.NewBuildOutput()

	// Print header
	buildUI.PrintHeader(version)

	// Print build start
	buildUI.PrintBuildStart(len(files))

	// Build each file
	success := true
	var lastError error

	for _, file := range files {
		if err := buildFile(file, output, buildUI); err != nil {
			success = false
			lastError = err
			buildUI.PrintError(err.Error())
			break
		}
	}

	// Print summary
	if success {
		buildUI.PrintSummary(true, "")
		if watch {
			fmt.Println()
			buildUI.PrintInfo("Watch mode not yet implemented")
		}
	} else {
		buildUI.PrintSummary(false, lastError.Error())
		return lastError
	}

	return nil
}

func buildFile(inputPath string, outputPath string, buildUI *ui.BuildOutput) error {
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

	// Step 1: Parse
	parseStart := time.Now()
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, inputPath, src, 0)
	parseDuration := time.Since(parseStart)

	if err != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Parse",
			Status:   ui.StepError,
			Duration: parseDuration,
		})
		return fmt.Errorf("parse error: %w", err)
	}

	buildUI.PrintStep(ui.Step{
		Name:     "Parse",
		Status:   ui.StepSuccess,
		Duration: parseDuration,
	})

	// Step 2: Transform (skipped for now)
	buildUI.PrintStep(ui.Step{
		Name:    "Transform",
		Status:  ui.StepSkipped,
		Message: "no plugins enabled",
	})

	// Step 3: Generate
	genStart := time.Now()
	gen := generator.New(fset)
	outputCode, err := gen.Generate(file)
	genDuration := time.Since(genStart)

	if err != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Generate",
			Status:   ui.StepError,
			Duration: genDuration,
		})
		return fmt.Errorf("generation error: %w", err)
	}

	buildUI.PrintStep(ui.Step{
		Name:     "Generate",
		Status:   ui.StepSuccess,
		Duration: genDuration,
	})

	// Step 4: Write
	writeStart := time.Now()
	if err := os.WriteFile(outputPath, outputCode, 0644); err != nil {
		writeDuration := time.Since(writeStart)
		buildUI.PrintStep(ui.Step{
			Name:     "Write",
			Status:   ui.StepError,
			Duration: writeDuration,
		})
		return fmt.Errorf("failed to write output: %w", err)
	}
	writeDuration := time.Since(writeStart)

	buildUI.PrintStep(ui.Step{
		Name:     "Write",
		Status:   ui.StepSuccess,
		Duration: writeDuration,
		Message:  fmt.Sprintf("%d bytes written", len(outputCode)),
	})

	return nil
}

func runDingoFile(inputPath string, programArgs []string) error {
	// Create beautiful output
	buildUI := ui.NewBuildOutput()

	// Print minimal header for run mode
	buildUI.PrintHeader(version)
	fmt.Println()

	// Determine output path
	outputPath := ""
	if len(inputPath) > 6 && inputPath[len(inputPath)-6:] == ".dingo" {
		outputPath = inputPath[:len(inputPath)-6] + ".go"
	} else {
		outputPath = inputPath + ".go"
	}

	// Step 1: Build (transpile)
	buildStart := time.Now()

	// Read source
	src, err := os.ReadFile(inputPath)
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Failed to read %s: %v", inputPath, err))
		return err
	}

	// Parse
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, inputPath, src, 0)
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Parse error: %v", err))
		return err
	}

	// Generate
	gen := generator.New(fset)
	goCode, err := gen.Generate(file)
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Generation error: %v", err))
		return err
	}

	// Write
	if err := os.WriteFile(outputPath, goCode, 0644); err != nil {
		buildUI.PrintError(fmt.Sprintf("Failed to write %s: %v", outputPath, err))
		return err
	}

	buildDuration := time.Since(buildStart)

	// Show build status
	fmt.Printf("  📝 Compiled %s → %s (%s)\n",
		filepath.Base(inputPath),
		filepath.Base(outputPath),
		formatDuration(buildDuration))
	fmt.Println()

	// Step 2: Run with go run
	fmt.Println("  🚀 Running...")
	fmt.Println()

	// Prepare go run command
	cmdArgs := []string{"run", outputPath}
	cmdArgs = append(cmdArgs, programArgs...)

	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Run and get exit code
	err = cmd.Run()

	fmt.Println()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Program ran but exited with error
			os.Exit(exitErr.ExitCode())
		}
		// Failed to run
		buildUI.PrintError(fmt.Sprintf("Failed to run: %v", err))
		return err
	}

	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	} else if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}
