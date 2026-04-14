package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed skill_embed/SKILL.md
var embeddedSkill embed.FS

var skillDirs = []string{
	".config/opencode/skills/stenographer",
	".claude/skills/stenographer",
	".agents/skills/stenographer",
}

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the stenographer skill file for AI coding tools",
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the stenographer skill file into global skill directories",
	Long: `Installs SKILL.md into:
  ~/.config/opencode/skills/stenographer/SKILL.md
  ~/.claude/skills/stenographer/SKILL.md
  ~/.agents/skills/stenographer/SKILL.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		content, err := embeddedSkill.ReadFile("skill_embed/SKILL.md")
		if err != nil {
			return fmt.Errorf("read embedded skill: %w", err)
		}

		for _, rel := range skillDirs {
			dir := filepath.Join(home, rel)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "skip %s: %v\n", dir, err)
				continue
			}
			path := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(path, content, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "skip %s: %v\n", path, err)
				continue
			}
			fmt.Printf("installed %s\n", path)
		}
		return nil
	},
}

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the stenographer skill file from global skill directories",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		for _, rel := range skillDirs {
			dir := filepath.Join(home, rel)
			path := filepath.Join(dir, "SKILL.md")
			if err := os.Remove(path); err != nil {
				if !os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "skip %s: %v\n", path, err)
				}
				continue
			}
			fmt.Printf("removed %s\n", path)
			// Remove the directory if empty.
			os.Remove(dir)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillUninstallCmd)
}
