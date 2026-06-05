package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
)

var osExecutable = os.Executable
var copyFileFn = copyFile
var filepathWalk = filepath.Walk

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "Install the bundled AI Agent Skill",
	RunE:  runInstallSkill,
}

func init() {
	rootCmd.AddCommand(installSkillCmd)
}

// findSkillsDir resolves bundled skill files: beside the binary (release tarball),
// parent of bin/ (npm: packageRoot/bin/jira-cli → packageRoot/skills), or cwd ./skills (dev).
func findSkillsDir(execDir string) string {
	candidates := []string{
		filepath.Join(execDir, "skills"),
		filepath.Join(execDir, "..", "skills"),
	}
	for _, dir := range candidates {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	if fi, err := os.Stat("skills"); err == nil && fi.IsDir() {
		return "skills"
	}
	return ""
}

func runInstallSkill(cmd *cobra.Command, args []string) error {
	execPath, err := osExecutable()
	if err != nil {
		output.Error("failed to locate binary: " + err.Error())
		return SilentErr(ExitBadArgs)
	}
	execDir := filepath.Dir(execPath)
	skillsDir := findSkillsDir(execDir)
	if skillsDir == "" {
		output.Error("no skill files found to install")
		return SilentErr(ExitBadArgs)
	}

	// Target directory
	home, err := os.UserHomeDir()
	if err != nil {
		output.Error("failed to get home directory: " + err.Error())
		return SilentErr(ExitBadArgs)
	}
	targetDir := filepath.Join(home, ".openclaw", "skills")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		output.Error("failed to create target directory: " + err.Error())
		return SilentErr(ExitBadArgs)
	}

	// Walk and copy files
	var installedFiles []string
	err = filepathWalk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(skillsDir, path)
		destPath := filepath.Join(targetDir, rel)

		// Ensure target subdirectory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// Check if already exists
		_, existErr := os.Stat(destPath)
		updated := existErr == nil

		// Copy file
		if err := copyFileFn(path, destPath); err != nil {
			return fmt.Errorf("copying %s: %w", rel, err)
		}

		installedFiles = append(installedFiles, rel)
		if !jsonMode {
			if updated {
				output.Info(fmt.Sprintf("Updated: %s", rel))
			} else {
				output.Success(fmt.Sprintf("Installed: %s", rel))
			}
		}
		return nil
	})

	if err != nil {
		output.Error("install failed: " + err.Error())
		return SilentErr(ExitBadArgs)
	}

	if len(installedFiles) == 0 {
		output.Error("no skill files found to install")
		return SilentErr(ExitBadArgs)
	}

	if jsonMode {
		output.PrintJSON(map[string]any{
			"installedFiles": installedFiles,
			"targetDir":      targetDir,
		})
		return nil
	}

	fmt.Println()
	output.Success(fmt.Sprintf("Skill installed to %s", targetDir))
	output.Gray("  AI Agents will now have access to jira-cli capabilities.")
	fmt.Println()
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}
