package main

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
)

// GitHubRelease represents the structure we need from GitHub API
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

func ctlHandleUpgradeCommand(args []string) {
	force := false
	for _, arg := range args {
		if arg == "--force" || arg == "-f" {
			force = true
		}
	}

	fmt.Printf("Checking for updates... (Current version: %s)\n", Version)

	// 1. Get latest version
	latestRelease, err := getLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching latest version: %v\n", err)
		os.Exit(1)
	}

	latestVersion := latestRelease.TagName
	if !force {
		// Handle "v" prefix inconsistencies
		cleanCurrent := strings.TrimPrefix(Version, "v")
		cleanLatest := strings.TrimPrefix(latestVersion, "v")

		// Simple string comparison for now, assuming semantic versioning
		if cleanCurrent == cleanLatest {
			fmt.Printf("Diane is already up to date (%s).\n", Version)
			return
		}
	}

	fmt.Printf("Found new version: %s\n", latestVersion)
	fmt.Println("Upgrading...")

	// 2. Detect platform
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)

	// Validate platform against supported ones
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "Unsupported OS for automatic upgrade: %s\n", runtime.GOOS)
		os.Exit(1)
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		fmt.Fprintf(os.Stderr, "Unsupported architecture for automatic upgrade: %s\n", runtime.GOARCH)
		os.Exit(1)
	}

	// 3. Construct URL
	// https://github.com/diane-assistant/diane/releases/download/v1.0.0/diane-darwin-arm64.tar.gz
	downloadURL := fmt.Sprintf("https://github.com/diane-assistant/diane/releases/download/%s/diane-%s.tar.gz", latestVersion, platform)

	// 4. Download and Extract
	tmpDir, err := os.MkdirTemp("", "diane-upgrade")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Downloading from %s...\n", downloadURL)

	tarballPath := filepath.Join(tmpDir, "diane.tar.gz")
	if err := downloadFile(downloadURL, tarballPath); err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Extracting...")
	binaryPath, err := extractBinary(tarballPath, tmpDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Extraction failed: %v\n", err)
		os.Exit(1)
	}

	// 5. Replace Binary
	currentExe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error determining executable path: %v\n", err)
		os.Exit(1)
	}

	// Resolve symlinks to find the real binary
	realExe, err := filepath.EvalSymlinks(currentExe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving symlinks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installing to %s...\n", realExe)

	// Move current binary to backup
	backupExe := realExe + ".old"
	if err := os.Rename(realExe, backupExe); err != nil {
		// Try to handle permission denied or other errors
		if os.IsPermission(err) {
			fmt.Fprintf(os.Stderr, "Permission denied. Please run with sudo:\n  sudo diane upgrade\n")
		} else {
			fmt.Fprintf(os.Stderr, "Error moving current binary: %v\n", err)
		}
		os.Exit(1)
	}

	// Move new binary to location
	// We use copyFile instead of Rename because tmpDir might be on a different filesystem
	if err := copyFile(binaryPath, realExe); err != nil {
		// Restore backup
		os.Rename(backupExe, realExe)
		fmt.Fprintf(os.Stderr, "Error installing new binary: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chmod(realExe, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to chmod new binary: %v\n", err)
	}

	// Cleanup backup
	os.Remove(backupExe)

	fmt.Printf("Successfully upgraded to %s\n", latestVersion)

	// Verify the upgrade by checking the version
	fmt.Println("\nVerifying installation...")
	verifyUpgrade(realExe, latestVersion)

	// If running on Linux, restart the daemon
	if runtime.GOOS == "linux" {
		restartLinuxDaemon()
	}
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

		// We are looking for the 'diane' binary
		cleanName := filepath.Base(header.Name)
		if cleanName == "diane" {
			destPath := filepath.Join(destDir, "diane-new")

			outFile, err := os.Create(destPath)
			if err != nil {
				return "", err
			}

			// Copy allows for limited memory usage vs ReadAll
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()

			// Ensure it's executable
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

// verifyUpgrade checks the installed binary version and validates it's correct
func verifyUpgrade(binaryPath, expectedVersion string) {
	cmd := exec.Command(binaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not verify installation: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please run 'diane version' manually to verify.\n")
		return
	}

	installedVersion := strings.TrimSpace(string(output))
	// Handle "diane v1.13.2" or just "v1.13.2"
	installedVersion = strings.TrimPrefix(installedVersion, "diane ")
	cleanExpected := strings.TrimPrefix(expectedVersion, "v")
	cleanInstalled := strings.TrimPrefix(installedVersion, "v")

	if cleanInstalled == cleanExpected {
		fmt.Printf("✓ Version verified: %s\n", installedVersion)
	} else {
		fmt.Fprintf(os.Stderr, "⚠ Warning: Version mismatch!\n")
		fmt.Fprintf(os.Stderr, "  Expected: %s\n", expectedVersion)
		fmt.Fprintf(os.Stderr, "  Installed: %s\n", installedVersion)

		// Check if there might be another binary in PATH
		pathBinary, err := exec.LookPath("diane")
		if err == nil && pathBinary != binaryPath {
			resolvedPath, _ := filepath.EvalSymlinks(pathBinary)
			fmt.Fprintf(os.Stderr, "\n⚠ Multiple diane binaries detected:\n")
			fmt.Fprintf(os.Stderr, "  Upgraded: %s\n", binaryPath)
			fmt.Fprintf(os.Stderr, "  In PATH:  %s\n", resolvedPath)
			fmt.Fprintf(os.Stderr, "\nConsider removing the old binary or updating your PATH.\n")
		}
	}
}

// restartLinuxDaemon attempts to restart the diane systemd service on Linux
func restartLinuxDaemon() {
	fmt.Println("\nDetecting Linux daemon...")

	// Check if systemd is available
	if _, err := exec.LookPath("systemctl"); err != nil {
		fmt.Println("systemctl not found, skipping daemon restart")
		return
	}

	// Check if diane service exists
	cmd := exec.Command("systemctl", "status", "diane")
	if err := cmd.Run(); err != nil {
		fmt.Println("diane.service not found or not running, skipping restart")
		return
	}

	fmt.Println("Attempting to restart diane.service...")

	// Try to restart the service
	cmd = exec.Command("systemctl", "restart", "diane")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ Warning: Failed to restart daemon: %v\n", err)
		fmt.Fprintf(os.Stderr, "You may need to restart manually:\n")
		fmt.Fprintf(os.Stderr, "  sudo systemctl restart diane\n")
		return
	}

	fmt.Println("✓ diane.service restarted successfully")

	// Give it a moment to start
	time.Sleep(2 * time.Second)

	// Check status
	cmd = exec.Command("systemctl", "is-active", "diane")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ Service may not have started properly\n")
		fmt.Fprintf(os.Stderr, "Check status with: systemctl status diane\n")
		return
	}

	status := strings.TrimSpace(string(output))
	if status == "active" {
		fmt.Println("✓ diane.service is running")
	} else {
		fmt.Fprintf(os.Stderr, "⚠ Service status: %s\n", status)
		fmt.Fprintf(os.Stderr, "Check logs with: journalctl -u diane -n 50\n")
	}
}
