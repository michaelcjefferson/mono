package tlscerts

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"placeholder_project_tag/pkg/logging"
)

// installMkcert downloads and installs mkcert if not found
func installMkcert(downloadURL, installPath string, logger *logging.Logger) error {
	logger.Info("mkcert not found, installing...", nil)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download mkcert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response downloading mkcert: %s", resp.Status)
	}

	out, err := os.Create(installPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to save mkcert: %w", err)
	}

	// Make executable on Linux/macOS
	if runtime.GOOS != "windows" {
		if err := os.Chmod(installPath, 0755); err != nil {
			return fmt.Errorf("failed to make mkcert executable: %w", err)
		}
	}

	logger.Info("mkcert installed successfully", nil)
	return nil
}

// isRootCAInstalled checks if the root CA is already installed
func isRootCAInstalled(mkcertPath string) bool {
	cmd := exec.Command(mkcertPath, "-CAROOT")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	caRoot := strings.TrimSpace(string(output))
	rootCert := filepath.Join(caRoot, "rootCA.pem")
	rootKey := filepath.Join(caRoot, "rootCA-key.pem")

	// Check if both rootCA files exist
	if _, err := os.Stat(rootCert); err != nil {
		return false
	}
	if _, err := os.Stat(rootKey); err != nil {
		return false
	}

	// Optional: Check if the rootCA is trusted by the system
	return isCATrusted(rootCert)
}

func isCATrusted(certPath string) bool {
	switch runtime.GOOS {
	case "darwin":
		// macOS: Use `security` to check if cert is in System keychain
		cmd := exec.Command("security", "find-certificate", "-c", "mkcert development CA")
		err := cmd.Run()
		return err == nil

	case "linux":
		// Linux: check if cert has been linked into /etc/ssl or /usr/local/share/ca-certificates
		// or try running update-ca-certificates --fresh
		_, err := os.ReadFile(certPath)
		if err != nil {
			return false
		}

		// Check if the cert exists in the system's trust store
		cmd := exec.Command("openssl", "verify", "-CAfile", "/etc/ssl/certs/ca-certificates.crt", certPath)
		err = cmd.Run()
		return err == nil

	case "windows":
		// Windows: use certutil to search the root store
		cmd := exec.Command("certutil", "-verifystore", "root", "mkcert development CA")
		output, err := cmd.CombinedOutput()
		return err == nil && strings.Contains(string(output), "mkcert development CA")

	default:
		// Unsupported OS
		return false
	}
}

// generateSSLCert runs mkcert to create a trusted certificate pair in the provided TLS directory path
func GenerateTLSCert(tlsDirPath, ip string, logger *logging.Logger) error {
	var downloadURL, mkcertPath string
	// TODO: Add architecture checks
	switch runtime.GOOS {
	case "windows":
		downloadURL = "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-windows-amd64.exe"
		mkcertPath = filepath.Join(os.Getenv("LocalAppData"), "Programs", "kamar-listener", "mkcert.exe")
	case "linux":
		downloadURL = "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-linux-amd64"
		mkcertPath = "/usr/local/bin/mkcert"
	case "darwin":
		downloadURL = "https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-darwin-arm64"
		mkcertPath = "/usr/local/bin/mkcert"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	// Check if mkcert is installed
	if _, err := exec.LookPath("mkcert"); err == nil {
		mkcertPath = "mkcert"
	} else {
		if _, err := os.Stat(mkcertPath); os.IsNotExist(err) {
			if err := installMkcert(downloadURL, mkcertPath, logger); err != nil {
				return err
			}
		}
	}

	// // Check if mkcert is installed
	// if _, err := exec.LookPath("mkcert"); err != nil {
	// 	if err := installMkcert(downloadURL, mkcertPath, logger); err != nil {
	// 		return err
	// 	}
	// } else {
	// 	mkcertPath = "mkcert"
	// }

	// Install mkcert root CA
	if !isRootCAInstalled(mkcertPath) {
		logger.Info("Running mkcert -install...", nil)
		cmd := exec.Command(mkcertPath, "-install")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run mkcert -install: %w", err)
		}
	}

	// Ensure the TLS directory exists
	if err := os.MkdirAll(tlsDirPath, 0755); err != nil {
		return fmt.Errorf("failed to create TLS directory: %w", err)
	}

	// Define certificate paths
	certPath := filepath.Join(tlsDirPath, "cert.pem")
	keyPath := filepath.Join(tlsDirPath, "key.pem")

	// Check if cert already exists
	if _, err := os.Stat(certPath); err == nil {
		logger.Info("TLS certificate already exists, skipping generation.", nil)
		return nil
	}

	// Generate SSL certificate
	logger.Info("Generating localhost TLS certificate...", nil)
	cmd := exec.Command(mkcertPath, "-key-file", keyPath, "-cert-file", certPath, "localhost", ip)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate certificate: %w", err)
	}

	logger.Info("TLS certificate generated successfully: key.pem & cert.pem", nil)
	return nil
}
