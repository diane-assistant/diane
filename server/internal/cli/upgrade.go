package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// GitHubRelease represents the structure from the GitHub API
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")

			fmt.Printf("Checking for updates... (Current version: %s)\n", cliVersion)

			latestRelease, err := getLatestRelease()
			if err != nil {
				return fmt.Errorf("error fetching latest version: %w", err)
			}

			latestVersion := latestRelease.TagName
			if !force {
				cleanCurrent := strings.TrimPrefix(cliVersion, "v")
				cleanLatest := strings.TrimPrefix(latestVersion, "v")
				if cleanCurrent == cleanLatest {
					PrintSuccess(fmt.Sprintf("Diane is already up to date (%s).", cliVersion))
					return nil
				}
			}

			fmt.Printf("Found new version: %s\n", latestVersion)
			fmt.Println("Upgrading...")

			// Detect platform
			platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
			if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
				return fmt.Errorf("unsupported OS for automatic upgrade: %s", runtime.GOOS)
			}
			if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
				return fmt.Errorf("unsupported architecture for automatic upgrade: %s", runtime.GOARCH)
			}

			// Construct URL
			downloadURL := fmt.Sprintf("https://github.com/diane-assistant/diane/releases/download/%s/diane-%s.tar.gz", latestVersion, platform)

			// Download and extract
			tmpDir, err := os.MkdirTemp("", "diane-upgrade")
			if err != nil {
				return fmt.Errorf("error creating temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			fmt.Printf("Downloading from %s...\n", downloadURL)

			tarballPath := filepath.Join(tmpDir, "diane.tar.gz")
			if err := downloadFile(downloadURL, tarballPath); err != nil {
				return fmt.Errorf("download failed: %w", err)
			}

			fmt.Println("Extracting...")
			binaryPath, err := extractBinary(tarballPath, tmpDir)
			if err != nil {
				return fmt.Errorf("extraction failed: %w", err)
			}

			// Replace binary
			currentExe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("error determining executable path: %w", err)
			}

			realExe, err := filepath.EvalSymlinks(currentExe)
			if err != nil {
				return fmt.Errorf("error resolving symlinks: %w", err)
			}

			fmt.Printf("Installing to %s...\n", realExe)

			backupExe := realExe + ".old"
			if err := os.Rename(realExe, backupExe); err != nil {
				if os.IsPermission(err) {
					return fmt.Errorf("permission denied. Please run with sudo:\n  sudo diane upgrade")
				}
				return fmt.Errorf("error moving current binary: %w", err)
			}

			if err := copyFile(binaryPath, realExe); err != nil {
				os.Rename(backupExe, realExe)
				return fmt.Errorf("error installing new binary: %w", err)
			}

			if err := os.Chmod(realExe, 0755); err != nil {
				PrintWarning(fmt.Sprintf("failed to chmod new binary: %v", err))
			}

			os.Remove(backupExe)

			PrintSuccess(fmt.Sprintf("Successfully upgraded to %s", latestVersion))

			// Verify
			fmt.Println("\nVerifying installation...")
			verifyUpgrade(realExe, latestVersion)

			if runtime.GOOS == "linux" {
				restartLinuxDaemon()
			}

			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "Force upgrade even if already at latest version")
	return cmd
}

func getLatestRelease() (*GitHubRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/diane-assistant/diane/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(tarballPath, destDir string) (string, error) {
	f, err := os.Open(tarballPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		cleanName := filepath.Base(header.Name)
		if cleanName == "diane" {
			destPath := filepath.Join(destDir, "diane-new")
			outFile, err := os.Create(destPath)
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()

			os.Chmod(destPath, 0755)
			return destPath, nil
		}
	}
	return "", fmt.Errorf("binary 'diane' not found in archive")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func verifyUpgrade(binaryPath, expectedVersion string) {
	cmd := exec.Command(binaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		PrintWarning(fmt.Sprintf("Could not verify installation: %v", err))
		fmt.Fprintln(os.Stderr, "Please run 'diane version' manually to verify.")
		return
	}

	installedVersion := strings.TrimSpace(string(output))
	installedVersion = strings.TrimPrefix(installedVersion, "diane ")
	cleanExpected := strings.TrimPrefix(expectedVersion, "v")
	cleanInstalled := strings.TrimPrefix(installedVersion, "v")

	if cleanInstalled == cleanExpected {
		PrintSuccess(fmt.Sprintf("Version verified: %s", installedVersion))
	} else {
		PrintWarning("Version mismatch!")
		fmt.Fprintf(os.Stderr, "  Expected: %s\n", expectedVersion)
		fmt.Fprintf(os.Stderr, "  Installed: %s\n", installedVersion)

		pathBinary, err := exec.LookPath("diane")
		if err == nil && pathBinary != binaryPath {
			resolvedPath, _ := filepath.EvalSymlinks(pathBinary)
			PrintWarning("Multiple diane binaries detected:")
			fmt.Fprintf(os.Stderr, "  Upgraded: %s\n", binaryPath)
			fmt.Fprintf(os.Stderr, "  In PATH:  %s\n", resolvedPath)
			fmt.Fprintln(os.Stderr, "\nConsider removing the old binary or updating your PATH.")
		}
	}
}

func restartLinuxDaemon() {
	fmt.Println("\nDetecting Linux daemon...")

	if _, err := exec.LookPath("systemctl"); err != nil {
		fmt.Println("systemctl not found, skipping daemon restart")
		return
	}

	cmd := exec.Command("systemctl", "status", "diane")
	if err := cmd.Run(); err != nil {
		fmt.Println("diane.service not found or not running, skipping restart")
		return
	}

	fmt.Println("Attempting to restart diane.service...")

	cmd = exec.Command("systemctl", "restart", "diane")
	if err := cmd.Run(); err != nil {
		PrintWarning(fmt.Sprintf("Failed to restart daemon: %v", err))
		fmt.Fprintln(os.Stderr, "You may need to restart manually:")
		fmt.Fprintln(os.Stderr, "  sudo systemctl restart diane")
		return
	}

	PrintSuccess("diane.service restarted successfully")

	time.Sleep(2 * time.Second)

	cmd = exec.Command("systemctl", "is-active", "diane")
	output, err := cmd.Output()
	if err != nil {
		PrintWarning("Service may not have started properly")
		fmt.Fprintln(os.Stderr, "Check status with: systemctl status diane")
		return
	}

	status := strings.TrimSpace(string(output))
	if status == "active" {
		PrintSuccess("diane.service is running")
	} else {
		PrintWarning(fmt.Sprintf("Service status: %s", status))
		fmt.Fprintln(os.Stderr, "Check logs with: journalctl -u diane -n 50")
	}
}
