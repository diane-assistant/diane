package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/diane-assistant/diane/internal/api"
)

// ctlHandleSlaveCommand handles slave management commands
func ctlHandleSlaveCommand(client *api.Client, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: slave command requires a subcommand")
		fmt.Fprintln(os.Stderr, "\nAvailable subcommands:")
		fmt.Fprintln(os.Stderr, "  pair <master-url>     Initiate pairing with master (run on slave)")
		fmt.Fprintln(os.Stderr, "  start                 Start the slave daemon (run on slave)")
		fmt.Fprintln(os.Stderr, "  pending               List pending pairing requests (run on master)")
		fmt.Fprintln(os.Stderr, "  approve <hostname> <code 123-456>  Approve a pairing request (run on master)")
		fmt.Fprintln(os.Stderr, "  deny <hostname> <code 123-456>     Deny a pairing request (run on master)")
		fmt.Fprintln(os.Stderr, "  list                  List all registered slaves (run on master)")
		fmt.Fprintln(os.Stderr, "  revoke <hostname>     Revoke slave credentials (run on master)")
		fmt.Fprintln(os.Stderr, "  revoked               List revoked credentials (run on master)")
		os.Exit(1)
	}

	subcommand := args[0]

	switch subcommand {
	// --- Slave-side commands ---
	case "pair":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: pair command requires master URL")
			fmt.Fprintln(os.Stderr, "Usage: diane slave pair <master-url>")
			os.Exit(1)
		}
		ctlSlavePair(args[1])

	case "start":
		ctlSlaveStart()

	// --- Master-side commands ---
	case "pending":
		ctlSlavePending(client)

	case "approve":
		if len(args) < 2 {
			ctlSlaveApproveInteractive(client)
			return
		}
		// Allow "123 456" as pairing code (handle space)
		code := args[2]
		if len(args) > 3 {
			code = args[2] + " " + args[3]
		}
		// Normalize code
		code = strings.ReplaceAll(code, " ", "-")
		if !strings.Contains(code, "-") && len(code) == 6 {
			code = code[:3] + "-" + code[3:]
		}
		ctlSlaveApprove(client, args[1], code)

	case "deny":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: deny command requires hostname and pairing code")
			fmt.Fprintln(os.Stderr, "Usage: diane slave deny <hostname> <code 123-456>")
			os.Exit(1)
		}
		// Allow "123 456" as pairing code (handle space)
		code := args[2]
		if len(args) > 3 {
			code = args[2] + " " + args[3]
		}
		// Normalize code
		code = strings.ReplaceAll(code, " ", "-")
		if !strings.Contains(code, "-") && len(code) == 6 {
			code = code[:3] + "-" + code[3:]
		}
		ctlSlaveDeny(client, args[1], code)

	case "list":
		ctlSlaveList(client)

	case "revoke":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: revoke command requires hostname")
			fmt.Fprintln(os.Stderr, "Usage: diane slave revoke <hostname>")
			os.Exit(1)
		}
		ctlSlaveRevoke(client, args[1])

	case "revoked":
		ctlSlaveRevoked(client)

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown slave subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

// ctlSlavePair initiates pairing with a master Diane instance
func ctlSlavePair(masterURL string) {
	// Normalize URL
	if !strings.HasPrefix(masterURL, "http://") && !strings.HasPrefix(masterURL, "https://") {
		masterURL = "http://" + masterURL
	}
	u, err := url.Parse(masterURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid master URL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Initiating pairing with master: %s\n", u.String())

	// 1. Generate key pair
	fmt.Println("Generating key pair...")
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
		os.Exit(1)
	}

	// 2. Generate CSR
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
		fmt.Fprintf(os.Stderr, "Error creating CSR: %v\n", err)
		os.Exit(1)
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	// 3. Send request to master
	reqBody := map[string]string{
		"hostname": hostname,
		"csr":      string(csrPEM),
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(u.String()+"/api/slaves/pair", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to master: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error from master: %s\n", string(body))
		os.Exit(1)
	}

	var pairResp struct {
		Success     bool   `json:"success"`
		Message     string `json:"message"`
		PairingCode string `json:"pairing_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pairResp); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding response: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nPairing request submitted successfully!")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("PAIRING CODE: %s\n", pairResp.PairingCode)
	fmt.Println("--------------------------------------------------")
	fmt.Println("\nPlease go to the master server and approve this request:")
	fmt.Printf("  diane slave approve %s %s\n", hostname, pairResp.PairingCode)
	fmt.Println("\nPlease go to the master server and approve this request:")
	fmt.Printf("  diane slave approve %s %s\n", hostname, pairResp.PairingCode)
	fmt.Println("\nWaiting for approval... (Press Ctrl+C to cancel)")

	// 4. Poll for approval
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		statusResp, err := http.Get(u.String() + "/api/slaves/pair/" + pairResp.PairingCode)
		if err != nil {
			// Ignore temporary errors
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
			fmt.Println("\n✅ Pairing approved!")

			// Save keys and certs
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

			// Read existing config or create new
			var cfgMap map[string]interface{}
			if data, err := os.ReadFile(configPath); err == nil {
				json.Unmarshal(data, &cfgMap)
			}
			if cfgMap == nil {
				cfgMap = make(map[string]interface{})
			}

			// Update slave config
			slaveCfg := map[string]interface{}{
				"enabled":    true,
				"master_url": masterURL,
			}
			cfgMap["slave"] = slaveCfg

			// Write back
			data, _ := json.MarshalIndent(cfgMap, "", "  ")
			os.WriteFile(configPath, data, 0644)
			fmt.Printf("Configuration updated at %s\n", configPath)

			// Start slave
			fmt.Println("\nStarting Diane in slave mode...")
			ctlSlaveStart()
			return
		} else if status.Status == "denied" {
			fmt.Println("\n❌ Pairing request denied by master.")
			os.Exit(1)
		}
	}
}

func ctlSlaveStart() {
	fmt.Println("Starting Diane daemon...")

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable: %v\n", err)
		os.Exit(1)
	}

	// Ensure we pass the 'serve' argument to start the daemon
	args := []string{"diane", "serve"}

	env := os.Environ()

	// Replace current process with daemon
	if err := syscall.Exec(exe, args, env); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}

// --- Master-side implementations ---

func ctlSlavePending(client *api.Client) {
	reqs, err := client.GetPendingPairingRequests()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(reqs) == 0 {
		fmt.Println("No pending pairing requests.")
		return
	}

	fmt.Println("Pending Pairing Requests:")
	fmt.Println("HOSTNAME          CODE       CREATED")
	for _, r := range reqs {
		created, _ := time.Parse(time.RFC3339, r.CreatedAt)
		fmt.Printf("%-17s %-10s %s\n", r.Hostname, r.PairingCode, created.Format(time.Kitchen))
	}
}

func ctlSlaveApprove(client *api.Client, hostname, code string) {
	resp, err := client.ApprovePairingRequest(hostname, code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Pairing approved successfully!")
	fmt.Println("Certificate issued.")

	// TODO: Display cert if we can't send it to slave
	if resp.Certificate != "" {
		fmt.Println("\n--- CERTIFICATE (Copy to slave: ~/.diane/slave-cert.pem) ---")
		fmt.Println(resp.Certificate)
		fmt.Println("-----------------------------------------------------------")
	}
}

func ctlSlaveApproveInteractive(client *api.Client) {
	reqs, err := client.GetPendingPairingRequests()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(reqs) == 0 {
		fmt.Println("No pending pairing requests.")
		return
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
			ctlSlaveApprove(client, req.Hostname, req.PairingCode)
		} else {
			fmt.Println("Cancelled.")
		}
	} else {
		fmt.Print("Enter number to approve (or 0 to cancel): ")
		var num int
		fmt.Scanln(&num)
		if num > 0 && num <= len(reqs) {
			req := reqs[num-1]
			ctlSlaveApprove(client, req.Hostname, req.PairingCode)
		} else {
			fmt.Println("Cancelled.")
		}
	}
}

func ctlSlaveDeny(client *api.Client, hostname, code string) {
	err := client.DenyPairingRequest(hostname, code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Pairing request denied.")
}

func ctlSlaveList(client *api.Client) {
	slaves, err := client.GetSlaves()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(slaves) == 0 {
		fmt.Println("No slaves registered.")
		return
	}

	fmt.Println("Registered Slaves:")
	fmt.Printf("%-20s %-12s %-5s %s\n", "HOSTNAME", "STATUS", "TOOLS", "LAST SEEN")
	for _, s := range slaves {
		lastSeen := "never"
		if s.LastSeen != "" {
			t, _ := time.Parse(time.RFC3339, s.LastSeen)
			lastSeen = t.Format(time.Kitchen)
		}
		fmt.Printf("%-20s %-12s %-5d %s\n", s.Hostname, s.Status, s.ToolCount, lastSeen)
	}
}

func ctlSlaveRevoke(client *api.Client, hostname string) {
	err := client.RevokeSlave(hostname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Credentials for %s revoked.\n", hostname)
}

func ctlSlaveRevoked(client *api.Client) {
	revoked, err := client.GetRevokedSlaves()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(revoked) == 0 {
		fmt.Println("No revoked credentials.")
		return
	}

	fmt.Println("Revoked Credentials:")
	fmt.Printf("%-20s %-20s %s\n", "HOSTNAME", "SERIAL", "REVOKED AT")
	for _, r := range revoked {
		t, _ := time.Parse(time.RFC3339, r.RevokedAt)
		fmt.Printf("%-20s %-20s %s\n", r.Hostname, r.CertSerial, t.Format(time.Kitchen))
	}
}
