package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var updateUnattended bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Stenographer to the latest release",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		current := Version
		fmt.Printf("Current version: %s\n", current)

		latest, err := fetchLatestVersion()
		if err != nil {
			return fmt.Errorf("check latest version: %w", err)
		}
		fmt.Printf("Latest version:  %s\n", latest)

		if latest == current {
			fmt.Println("Already up to date.")
			return nil
		}

		if !updateUnattended {
			fmt.Printf("Update to %s? [Y/n] ", latest)
			var confirm string
			fmt.Scanln(&confirm)
			confirm = strings.TrimSpace(confirm)
			if confirm != "" && confirm[0] != 'y' && confirm[0] != 'Y' {
				fmt.Println("Update cancelled.")
				return nil
			}
		}

		binaryName := fmt.Sprintf("stenographer-%s-%s", runtime.GOOS, runtime.GOARCH)
		url := fmt.Sprintf("https://github.com/nbitslabs/stenographer/releases/download/%s/%s", latest, binaryName)

		fmt.Printf("Downloading %s...\n", url)
		tmpFile, err := downloadRelease(url)
		if err != nil {
			return fmt.Errorf("download: %w", err)
		}
		defer os.Remove(tmpFile)

		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("determine executable path: %w", err)
		}

		if err := replaceExecutable(execPath, tmpFile); err != nil {
			return fmt.Errorf("replace binary: %w", err)
		}

		fmt.Printf("Updated to %s\n", latest)
		return nil
	},
}

func fetchLatestVersion() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/nbitslabs/stenographer/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no tag_name in response")
	}
	return release.TagName, nil
}

func downloadRelease(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "stenographer-update-*")
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()

	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	return tmp.Name(), nil
}

func replaceExecutable(dst, src string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Atomic-ish replace: rename new over old. On Unix this works even while
	// the binary is running because the old inode stays alive until the
	// process exits.
	if err := os.Rename(src, dst); err != nil {
		// rename may fail across filesystems; fall back to copy.
		return copyFile(dst, src)
	}
	return nil
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&updateUnattended, "unattended", false, "Skip confirmation prompt")
}
