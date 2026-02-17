package cli

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newSlaveCmd(client *api.Client) *cobra.Command {
	slaveCmd := &cobra.Command{
		Use:   "slave",
		Short: "Manage slave nodes (master/slave pairing)",
		Long:  "Commands for managing Diane master/slave relationships.",
	}

	slaveCmd.AddCommand(newSlavePairCmd())
	slaveCmd.AddCommand(newSlaveStartCmd())
	slaveCmd.AddCommand(newSlavePendingCmd(client))
	slaveCmd.AddCommand(newSlaveApproveCmd(client))
	slaveCmd.AddCommand(newSlaveDenyCmd(client))
	slaveCmd.AddCommand(newSlaveListCmd(client))
	slaveCmd.AddCommand(newSlaveRevokeCmd(client))
	slaveCmd.AddCommand(newSlaveRevokedCmd(client))

	return slaveCmd
}

// --- Slave-side commands ---

func newSlavePairCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pair <master-url>",
		Short: "Initiate pairing with master (run on slave)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			masterURL := args[0]

			// Normalize URL
			if !strings.HasPrefix(masterURL, "http://") && !strings.HasPrefix(masterURL, "https://") {
				masterURL = "https://" + masterURL
			}
			if strings.HasPrefix(masterURL, "http://") && strings.Contains(masterURL, ":8765") {
				masterURL = strings.Replace(masterURL, "http://", "https://", 1)
				masterURL = strings.Replace(masterURL, ":8765", ":8766", 1)
				PrintWarning("Upgrading connection to HTTPS on port 8766 for secure pairing")
			}

			u, err := url.Parse(masterURL)
			if err != nil {
				return fmt.Errorf("invalid master URL: %w", err)
			}

			fmt.Printf("Initiating pairing with master: %s\n", u.String())

			httpClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
				Timeout: 10 * time.Second,
			}

			// Generate key pair
			fmt.Println("Generating key pair...")
			key, err := rsa.GenerateKey(rand.Reader, 4096)
			if err != nil {
				return fmt.Errorf("error generating key: %w", err)
			}

			// Generate CSR
			hostname, _ := os.Hostname()
			fmt.Printf("Creating CSR for host: %s\n", hostname)

			csrTemplate := x509.CertificateRequest{
				Subject: pkix.Name{
					CommonName: hostname,
				},
				DNSNames: []string{hostname},
			}

			csrDER, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, key)
			if err != nil {
				return fmt.Errorf("error creating CSR: %w", err)
			}

			csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

			// Send request to master
			reqBody := map[string]string{
				"hostname": hostname,
				"csr":      string(csrPEM),
				"platform": runtime.GOOS,
			}
			jsonBody, _ := json.Marshal(reqBody)

			fmt.Println("\nConnecting to master...")
			var resp *http.Response

			for {
				resp, err = httpClient.Post(u.String()+"/api/slaves/pair", "application/json", bytes.NewBuffer(jsonBody))
				if err == nil {
					if resp.StatusCode == http.StatusOK {
						break
					}

					body, _ := io.ReadAll(resp.Body)
					resp.Body.Close()

					if resp.StatusCode == http.StatusNotFound {
						PrintError("Master returned 404 (Not Found).")
						fmt.Println("Possible causes:")
						fmt.Println("1. Master is running an old version (upgrade to v1.14.5+)")
						fmt.Println("2. You are connecting to the wrong port (try 8766 for secure pairing)")
						fmt.Println("3. API endpoint path is incorrect")
					} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
						PrintError(fmt.Sprintf("Master returned %d (Unauthorized/Forbidden).", resp.StatusCode))
						fmt.Println("You are likely connecting to the HTTP API port instead of the secure pairing port.")
						fmt.Println("Use HTTPS port 8766 for pairing: diane slave pair https://master-hostname:8766")
					} else {
						PrintError(fmt.Sprintf("Error from master: %s (Status %d)", string(body), resp.StatusCode))
					}
				} else {
					fmt.Printf("\r\033[KWaiting for master at %s... (%v)", u.String(), err)
				}

				fmt.Println("\n\nEnsure master is running:")
				fmt.Println("1. Check if 'diane serve' is running on the master machine")
				fmt.Println("2. Verify port 8766 is open and accessible")
				fmt.Println("3. Ensure master version is v1.14.5 or later")
				fmt.Println("\nRetrying in 5 seconds...")
				time.Sleep(5 * time.Second)
			}
			defer resp.Body.Close()

			var pairResp struct {
				Success     bool   `json:"success"`
				Message     string `json:"message"`
				PairingCode string `json:"pairing_code"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&pairResp); err != nil {
				return fmt.Errorf("error decoding response: %w", err)
			}

			PrintSuccess("Pairing request submitted successfully!")
			fmt.Println("--------------------------------------------------")
			fmt.Printf("PAIRING CODE: %s\n", pairResp.PairingCode)
			fmt.Println("--------------------------------------------------")
			fmt.Println("\nOn the master server, run:")
			fmt.Println("  diane slave approve")
			fmt.Println("\nThis will show pending requests. Verify the code matches")
			fmt.Println("and confirm to approve.")
			fmt.Println("\nWaiting for approval... (Press Ctrl+C to cancel)")

			// Poll for approval
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				statusResp, err := httpClient.Get(u.String() + "/api/slaves/pair/" + pairResp.PairingCode)
				if err != nil {
					continue
				}
				defer statusResp.Body.Close()

				if statusResp.StatusCode != http.StatusOK {
					continue
				}

				var status struct {
					Status      string `json:"status"`
					Certificate string `json:"certificate"`
					CACert      string `json:"ca_cert"`
				}
				if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
					continue
				}

				if status.Status == "approved" && status.Certificate != "" {
					PrintSuccess("Pairing approved!")

					home, _ := os.UserHomeDir()
					dianeDir := filepath.Join(home, ".diane")
					os.MkdirAll(dianeDir, 0755)

					// Save private key
					keyPath := filepath.Join(dianeDir, "slave-key.pem")
					keyOut, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
					pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
					keyOut.Close()
					fmt.Printf("Private key saved to %s\n", keyPath)

					// Save client cert
					certPath := filepath.Join(dianeDir, "slave-cert.pem")
					os.WriteFile(certPath, []byte(status.Certificate), 0644)
					fmt.Printf("Certificate saved to %s\n", certPath)

					// Save CA cert
					if status.CACert != "" {
						caPath := filepath.Join(dianeDir, "slave-ca-cert.pem")
						os.WriteFile(caPath, []byte(status.CACert), 0644)
						fmt.Printf("CA Certificate saved to %s\n", caPath)
					}

					// Update config
					configPath := filepath.Join(dianeDir, "config.json")
					var cfgMap map[string]interface{}
					if data, err := os.ReadFile(configPath); err == nil {
						json.Unmarshal(data, &cfgMap)
					}
					if cfgMap == nil {
						cfgMap = make(map[string]interface{})
					}
					slaveCfg := map[string]interface{}{
						"enabled":    true,
						"master_url": masterURL,
					}
					cfgMap["slave"] = slaveCfg
					data, _ := json.MarshalIndent(cfgMap, "", "  ")
					os.WriteFile(configPath, data, 0644)
					fmt.Printf("Configuration updated at %s\n", configPath)

					fmt.Println("\nStarting Diane in slave mode...")
					slaveStart()
					return nil
				} else if status.Status == "denied" {
					PrintError("Pairing request denied by master.")
					os.Exit(1)
				}
			}

			return nil
		},
	}
}

func newSlaveStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the slave daemon (run on slave)",
		RunE: func(cmd *cobra.Command, args []string) error {
			slaveStart()
			return nil
		},
	}
}

func slaveStart() {
	goos := runtime.GOOS

	// Check if Diane.app is running on macOS
	if goos == "darwin" {
		if isAppRunning("Diane") {
			fmt.Println("Restarting Diane.app to apply slave configuration...")
			if err := restartMacApp("Diane"); err != nil {
				PrintWarning(fmt.Sprintf("Failed to restart Diane.app: %v", err))
				fmt.Println("Please manually restart Diane.app to enable slave mode.")
			} else {
				PrintSuccess("Diane.app restarted successfully in slave mode!")
				fmt.Println("\nSlave is now connecting to master...")
			}
			return
		}
	}

	// Stop any conflicting diane processes
	fmt.Println("Checking for conflicting Diane processes...")
	stopConflictingProcesses()

	// Start daemon in background
	fmt.Println("Starting Diane daemon in background...")

	exe, err := os.Executable()
	if err != nil {
		PrintError(fmt.Sprintf("Error getting executable: %v", err))
		os.Exit(1)
	}

	slaveCmd := exec.Command(exe, "serve")
	slaveCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".diane", "slave.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		slaveCmd.Stdout = logFile
		slaveCmd.Stderr = logFile
		defer logFile.Close()
	}

	if err := slaveCmd.Start(); err != nil {
		PrintError(fmt.Sprintf("Error starting daemon: %v", err))
		os.Exit(1)
	}

	// Save PID
	pidPath := filepath.Join(home, ".diane", "diane.pid")
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", slaveCmd.Process.Pid)), 0644)

	PrintSuccess(fmt.Sprintf("Diane daemon started (PID: %d)", slaveCmd.Process.Pid))
	fmt.Printf("   Logs: %s\n", logPath)
	fmt.Println("\nSlave is now connecting to master...")

	time.Sleep(2 * time.Second)
}

func isAppRunning(appName string) bool {
	cmd := exec.Command("pgrep", "-f", fmt.Sprintf("/Applications/%s.app", appName))
	return cmd.Run() == nil
}

func restartMacApp(appName string) error {
	exec.Command("killall", appName).Run()
	time.Sleep(1 * time.Second)
	return exec.Command("open", "-a", appName).Run()
}

func stopConflictingProcesses() {
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		return
	}

	currentPid := os.Getpid()
	cmd := exec.Command("pgrep", "-u", currentUser, "diane")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid == currentPid {
			continue
		}

		psCmd := exec.Command("ps", "-p", pidStr, "-o", "command=")
		cmdlineBytes, err := psCmd.Output()
		if err != nil {
			continue
		}

		cmdline := string(cmdlineBytes)
		if strings.Contains(cmdline, "diane") && strings.Contains(cmdline, "serve") {
			fmt.Printf("Stopping conflicting diane process (PID: %d)...\n", pid)
			syscall.Kill(pid, syscall.SIGTERM)
		}
	}

	time.Sleep(1 * time.Second)
}

// --- Master-side commands ---

func newSlavePendingCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "pending",
		Short: "List pending pairing requests (run on master)",
		RunE: func(cmd *cobra.Command, args []string) error {
			reqs, err := client.GetPendingPairingRequests()
			if err != nil {
				return fmt.Errorf("error: %w", err)
			}

			if len(reqs) == 0 {
				fmt.Println("No pending pairing requests.")
				return nil
			}

			if tryJSON(cmd, reqs) {
				return nil
			}

			headers := []string{"HOSTNAME", "CODE", "CREATED"}
			var rows [][]string
			for _, r := range reqs {
				created, _ := time.Parse(time.RFC3339, r.CreatedAt)
				rows = append(rows, []string{r.Hostname, r.PairingCode, created.Format(time.Kitchen)})
			}
			RenderTable(headers, rows)
			return nil
		},
	}
}

func newSlaveApproveCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "approve [hostname] [code]",
		Short: "Approve a pairing request (run on master)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				// Interactive mode
				return slaveApproveInteractive(client)
			}

			hostname := args[0]
			if len(args) < 2 {
				return fmt.Errorf("approve command requires hostname and pairing code")
			}

			code := args[1]
			if len(args) > 2 {
				code = args[1] + " " + args[2]
			}
			code = normalizePairingCode(code)

			_, err := client.ApprovePairingRequest(hostname, code)
			if err != nil {
				return fmt.Errorf("error: %w", err)
			}

			PrintSuccess("Pairing approved successfully!")
			fmt.Printf("   Host: %s\n", hostname)
			fmt.Println("   Certificate issued and sent to slave")
			fmt.Println("\nThe slave should now be connecting to the master...")
			return nil
		},
	}
}

func slaveApproveInteractive(client *api.Client) error {
	reqs, err := client.GetPendingPairingRequests()
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	if len(reqs) == 0 {
		fmt.Println("No pending pairing requests.")
		return nil
	}

	fmt.Println("Pending Pairing Requests:")
	for i, r := range reqs {
		created, _ := time.Parse(time.RFC3339, r.CreatedAt)
		fmt.Printf("[%d] Host: %s, Code: %s, Time: %s\n", i+1, r.Hostname, r.PairingCode, created.Format(time.Kitchen))
	}

	fmt.Println()
	if len(reqs) == 1 {
		req := reqs[0]
		fmt.Printf("Approve request from %s (%s)? [Y/n]: ", req.Hostname, req.PairingCode)
		var response string
		fmt.Scanln(&response)
		if response == "" || strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
			_, err := client.ApprovePairingRequest(req.Hostname, req.PairingCode)
			if err != nil {
				return fmt.Errorf("error: %w", err)
			}
			PrintSuccess("Pairing approved successfully!")
			fmt.Printf("   Host: %s\n", req.Hostname)
			fmt.Println("   Certificate issued and sent to slave")
		} else {
			fmt.Println("Cancelled.")
		}
	} else {
		fmt.Print("Enter number to approve (or 0 to cancel): ")
		var num int
		fmt.Scanln(&num)
		if num > 0 && num <= len(reqs) {
			req := reqs[num-1]
			_, err := client.ApprovePairingRequest(req.Hostname, req.PairingCode)
			if err != nil {
				return fmt.Errorf("error: %w", err)
			}
			PrintSuccess("Pairing approved successfully!")
			fmt.Printf("   Host: %s\n", req.Hostname)
			fmt.Println("   Certificate issued and sent to slave")
		} else {
			fmt.Println("Cancelled.")
		}
	}

	return nil
}

func newSlaveDenyCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "deny <hostname> <code>",
		Short: "Deny a pairing request (run on master)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hostname := args[0]
			code := args[1]
			if len(args) > 2 {
				code = args[1] + " " + args[2]
			}
			code = normalizePairingCode(code)

			if err := client.DenyPairingRequest(hostname, code); err != nil {
				return fmt.Errorf("error: %w", err)
			}
			PrintSuccess("Pairing request denied.")
			return nil
		},
	}
}

func newSlaveListCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered slaves (run on master)",
		RunE: func(cmd *cobra.Command, args []string) error {
			slaves, err := client.GetSlaves()
			if err != nil {
				return fmt.Errorf("error: %w", err)
			}

			if len(slaves) == 0 {
				fmt.Println("No slaves registered.")
				return nil
			}

			if tryJSON(cmd, slaves) {
				return nil
			}

			headers := []string{"HOSTNAME", "STATUS", "TOOLS", "LAST SEEN"}
			var rows [][]string
			for _, s := range slaves {
				lastSeen := "never"
				if s.LastSeen != "" {
					t, _ := time.Parse(time.RFC3339, s.LastSeen)
					lastSeen = t.Format(time.Kitchen)
				}
				rows = append(rows, []string{s.Hostname, s.Status, fmt.Sprintf("%d", s.ToolCount), lastSeen})
			}
			RenderTable(headers, rows)
			return nil
		},
	}
}

func newSlaveRevokeCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <hostname>",
		Short: "Revoke slave credentials (run on master)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hostname := args[0]
			if err := client.RevokeSlave(hostname); err != nil {
				return fmt.Errorf("error: %w", err)
			}
			PrintSuccess(fmt.Sprintf("Credentials for %s revoked.", hostname))
			return nil
		},
	}
}

func newSlaveRevokedCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "revoked",
		Short: "List revoked credentials (run on master)",
		RunE: func(cmd *cobra.Command, args []string) error {
			revoked, err := client.GetRevokedSlaves()
			if err != nil {
				return fmt.Errorf("error: %w", err)
			}

			if len(revoked) == 0 {
				fmt.Println("No revoked credentials.")
				return nil
			}

			if tryJSON(cmd, revoked) {
				return nil
			}

			headers := []string{"HOSTNAME", "SERIAL", "REVOKED AT"}
			var rows [][]string
			for _, r := range revoked {
				t, _ := time.Parse(time.RFC3339, r.RevokedAt)
				rows = append(rows, []string{r.Hostname, r.CertSerial, t.Format(time.Kitchen)})
			}
			RenderTable(headers, rows)
			return nil
		},
	}
}

// normalizePairingCode normalizes the pairing code format (e.g., "123 456" -> "123-456")
func normalizePairingCode(code string) string {
	code = strings.ReplaceAll(code, " ", "-")
	if !strings.Contains(code, "-") && len(code) == 6 {
		code = code[:3] + "-" + code[3:]
	}
	return code
}
