package cli

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

	"github.com/mrz1836/sigil/internal/output"
	versionpkg "github.com/mrz1836/sigil/internal/version"
)

const (
	// devVersionString is the string used for development versions
	devVersionString = "dev"
	// upgradeOwner is the GitHub repository owner
	upgradeOwner = "mrz1836"
	// upgradeRepo is the GitHub repository name
	upgradeRepo = "sigil"
	// maxBinarySize is the maximum allowed binary size (200MB)
	maxBinarySize = 200 * 1024 * 1024
	// downloadTimeout is the HTTP timeout for downloads
	downloadTimeout = 60 * time.Second
)

var (
	// ErrDevVersionNoForce is returned when trying to upgrade a dev version without --force
	ErrDevVersionNoForce = errors.New("cannot upgrade development build without --force")
	// ErrVersionParseFailed is returned when version cannot be parsed from output
	ErrVersionParseFailed = errors.New("could not parse version from output")
	// ErrDownloadFailed is returned when binary download fails
	ErrDownloadFailed = errors.New("failed to download binary")
	// ErrChecksumMismatch is returned when SHA256 checksum verification fails
	ErrChecksumMismatch = errors.New("SHA256 checksum mismatch")
	// ErrChecksumNotFound is returned when the checksum entry is missing from the checksums file
	ErrChecksumNotFound = errors.New("checksum not found in checksums file")
	// ErrBinaryNotFoundInArchive is returned when the binary is not found in the archive
	ErrBinaryNotFoundInArchive = errors.New("sigil binary not found in archive")
	// ErrHTTPStatus is returned when an HTTP request returns a non-200 status code
	ErrHTTPStatus = errors.New("unexpected HTTP status")
)

// UpgradeConfig holds configuration for the upgrade command
type UpgradeConfig struct {
	Force        bool
	CheckOnly    bool
	UseGoInstall bool
}

// newUpgradeCmd creates the upgrade command
func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade sigil to the latest version",
		Long: `Upgrade sigil to the latest version available on GitHub.

This command will:
  - Check the latest version available on GitHub
  - Compare with the currently installed version
  - Download and verify the new binary (primary method)
  - Fall back to go install if binary download fails`,
		Example: `  # Check for available updates
  sigil upgrade --check

  # Upgrade to latest version
  sigil upgrade

  # Force upgrade even from a dev/commit build
  sigil upgrade --force

  # Prefer go install over binary download
  sigil upgrade --use-go-install`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := UpgradeConfig{}
			var err error

			cfg.Force, err = cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}

			cfg.CheckOnly, err = cmd.Flags().GetBool("check")
			if err != nil {
				return err
			}

			cfg.UseGoInstall, err = cmd.Flags().GetBool("use-go-install")
			if err != nil {
				return err
			}

			return runUpgradeWithConfig(cmd, cfg)
		},
	}

	// Add flags
	cmd.Flags().BoolP("force", "f", false, "Force upgrade even from a dev/commit build")
	cmd.Flags().Bool("check", false, "Check for updates without upgrading")
	cmd.Flags().Bool("use-go-install", false, "Prefer go install over binary download")

	return cmd
}

//nolint:gocyclo,gocognit // Upgrade flow has inherent branching complexity for version checks and fallback logic
func runUpgradeWithConfig(cmd *cobra.Command, cfg UpgradeConfig) error {
	currentVersion := GetCurrentVersion()

	// Handle development version or commit hash
	if currentVersion == devVersionString || currentVersion == "" || isLikelyCommitHash(currentVersion) {
		if !cfg.Force && !cfg.CheckOnly {
			output.Warn(fmt.Sprintf("Current version appears to be a development build (%s)", currentVersion))
			output.Info("Use --force to upgrade anyway")
			return ErrDevVersionNoForce
		}
	}

	output.Infof("Current version: %s", upgradeFormatVersion(currentVersion))

	// Fetch latest release
	output.Info("Checking for updates...")
	release, err := versionpkg.GetLatestRelease(cmd.Context(), upgradeOwner, upgradeRepo)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	output.Infof("Latest version: %s", upgradeFormatVersion(latestVersion))

	// Compare versions
	isNewer := versionpkg.IsNewerVersion(currentVersion, latestVersion)

	if !isNewer && !cfg.Force {
		output.Successf("You are already on the latest version (%s)", upgradeFormatVersion(currentVersion))
		return nil
	}

	if cfg.CheckOnly {
		if isNewer {
			output.Warnf("A newer version is available: %s -> %s", upgradeFormatVersion(currentVersion), upgradeFormatVersion(latestVersion))
			output.Info("Run 'sigil upgrade' to upgrade")
		} else {
			output.Success("You are on the latest version")
		}
		return nil
	}

	// Perform upgrade
	if isNewer {
		output.Infof("Upgrading from %s to %s...", upgradeFormatVersion(currentVersion), upgradeFormatVersion(latestVersion))
	} else if cfg.Force {
		output.Infof("Force reinstalling version %s...", upgradeFormatVersion(latestVersion))
	}

	// Perform upgrade using selected method with fallback
	if err := performUpgrade(cfg.UseGoInstall, latestVersion); err != nil {
		return err
	}

	output.Successf("Successfully upgraded to version %s", upgradeFormatVersion(latestVersion))

	return nil
}

// performUpgrade executes the upgrade using the preferred method with fallback.
func performUpgrade(useGoInstall bool, version string) error {
	if useGoInstall {
		if err := upgradeGoInstall(version); err != nil {
			output.Warn("go install failed, falling back to binary download...")
			if binErr := upgradeBinary(version); binErr != nil {
				return fmt.Errorf("both go install and binary upgrade methods failed: %w", binErr)
			}
		}
		return nil
	}
	if err := upgradeBinary(version); err != nil {
		output.Warn("Binary upgrade failed, falling back to go install...")
		if goErr := upgradeGoInstall(version); goErr != nil {
			return fmt.Errorf("both binary and go install upgrade methods failed: %w", goErr)
		}
	}
	return nil
}

// upgradeFormatVersion formats a version string for display
func upgradeFormatVersion(v string) string {
	if v == devVersionString || v == "" {
		return devVersionString
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// isLikelyCommitHash checks if a version string looks like a commit hash
//
//nolint:gocyclo,gocognit // Hex character validation requires explicit range checks
func isLikelyCommitHash(version string) bool {
	// Remove any -dirty suffix
	version = strings.TrimSuffix(version, "-dirty")

	// Commit hashes are typically 7-40 hex characters
	if len(version) < 7 || len(version) > 40 {
		return false
	}

	// Check if all characters are valid hex and at least one is a letter
	hasLetter := false
	for _, c := range version {
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		isUpperHex := c >= 'A' && c <= 'F'

		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
		if isLowerHex || isUpperHex {
			hasLetter = true
		}
	}

	return hasLetter
}

// GetCurrentVersion returns the current version of sigil
func GetCurrentVersion() string {
	v := buildInfo.Version
	if v == "" {
		return devVersionString
	}
	return v
}

// upgradeGoInstall upgrades using go install command
func upgradeGoInstall(latestVersion string) error {
	installPkg := fmt.Sprintf("github.com/mrz1836/sigil/cmd/sigil@v%s", latestVersion)

	output.Infof("Running: go install %s", installPkg)

	execCmd := exec.CommandContext(context.Background(), "go", "install", installPkg) //nolint:gosec // Package path is constructed from trusted version string
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}

	return nil
}

// upgradeBinary downloads and installs a pre-built binary from GitHub releases
func upgradeBinary(latestVersion string) error {
	// Get current binary location
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine current binary location: %w", err)
	}
	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("could not resolve binary symlinks: %w", err)
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "sigil-upgrade-*")
	if err != nil {
		return fmt.Errorf("could not create temporary directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Download and verify archive
	archiveName := fmt.Sprintf("%s_%s_%s_%s.tar.gz", upgradeRepo, latestVersion, runtime.GOOS, runtime.GOARCH)
	archivePath := filepath.Join(tempDir, archiveName)
	if err = downloadAndVerifyArchive(latestVersion, archiveName, archivePath); err != nil {
		return err
	}

	// Extract binary from tar.gz archive
	extractedBinary, err := extractBinaryFromArchive(archivePath, tempDir)
	if err != nil {
		return fmt.Errorf("could not extract binary: %w", err)
	}

	// Replace current binary (backup, copy, cleanup)
	return replaceBinary(currentBinary, extractedBinary)
}

// downloadAndVerifyArchive downloads the checksums and archive, then verifies the checksum.
func downloadAndVerifyArchive(latestVersion, archiveName, archivePath string) error {
	// Download checksums file
	checksumsURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/%s_%s_checksums.txt",
		upgradeOwner, upgradeRepo, latestVersion, upgradeRepo, latestVersion)

	output.Info("Downloading checksums...")
	checksums, err := downloadToString(checksumsURL)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}

	// Download the archive
	downloadURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/%s",
		upgradeOwner, upgradeRepo, latestVersion, archiveName)

	output.Infof("Downloading binary from: %s", downloadURL)
	if err := downloadToFile(downloadURL, archivePath); err != nil {
		return fmt.Errorf("%w: %w", ErrDownloadFailed, err)
	}

	// Verify SHA256 checksum
	output.Info("Verifying SHA256 checksum...")
	return verifyChecksum(archivePath, archiveName, checksums)
}

// replaceBinary backs up the current binary, copies the new one, and cleans up.
func replaceBinary(currentBinary, newBinary string) error {
	backupFile := currentBinary + ".backup"
	if err := os.Rename(currentBinary, backupFile); err != nil {
		return fmt.Errorf("could not backup current binary: %w", err)
	}

	if err := copyFile(newBinary, currentBinary, 0o755); err != nil {
		// Restore backup on failure
		_ = os.Rename(backupFile, currentBinary)
		return fmt.Errorf("could not replace binary: %w", err)
	}

	// Remove backup on success
	_ = os.Remove(backupFile)

	output.Info("Binary upgrade completed successfully")
	return nil
}

// downloadToString downloads a URL and returns its content as a string
func downloadToString(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req) //nolint:gosec // URL is constructed from trusted GitHub releases endpoint
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %d", ErrHTTPStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB max for checksums
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// downloadToFile downloads a URL to a local file
func downloadToFile(url, destPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req) //nolint:gosec // URL is constructed from trusted GitHub releases endpoint
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrHTTPStatus, resp.StatusCode)
	}

	file, err := os.Create(destPath) //nolint:gosec // Path is in controlled temp directory
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(file, io.LimitReader(resp.Body, maxBinarySize))
	return err
}

// verifyChecksum verifies the SHA256 checksum of a file against a checksums file
func verifyChecksum(filePath, fileName, checksums string) error {
	// Calculate SHA256 of downloaded file
	file, err := os.Open(filePath) //nolint:gosec // Path is in controlled temp directory
	if err != nil {
		return fmt.Errorf("could not open file for checksum: %w", err)
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("could not calculate checksum: %w", err)
	}
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	// Find expected checksum in checksums file
	expectedChecksum := findChecksumEntry(fileName, checksums)
	if expectedChecksum == "" {
		return fmt.Errorf("%w: %s", ErrChecksumNotFound, fileName)
	}

	if expectedChecksum != actualChecksum {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expectedChecksum, actualChecksum)
	}

	return nil
}

// findChecksumEntry looks up the checksum for a given file name in a checksums text.
func findChecksumEntry(fileName, checksums string) string {
	scanner := bufio.NewScanner(strings.NewReader(checksums))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) == 2 && parts[1] == fileName {
			return parts[0]
		}
	}
	return ""
}

// extractBinaryFromArchive extracts the sigil binary from a tar.gz archive file
func extractBinaryFromArchive(archivePath, destDir string) (string, error) {
	file, err := os.Open(archivePath) //nolint:gosec // Path is in controlled temp directory
	if err != nil {
		return "", fmt.Errorf("could not open archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("could not create gzip reader: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Extract files from tar
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("could not read tar entry: %w", err)
		}

		// Look for the sigil binary
		if filepath.Base(header.Name) == "sigil" && header.Typeflag == tar.TypeReg {
			return extractTarEntry(tarReader, destDir)
		}
	}

	return "", ErrBinaryNotFoundInArchive
}

// extractTarEntry writes a tar entry to the destination directory as the sigil binary.
func extractTarEntry(reader io.Reader, destDir string) (string, error) {
	destPath := filepath.Join(destDir, "sigil")
	outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // Need executable permissions
	if err != nil {
		return "", fmt.Errorf("could not create binary file: %w", err)
	}

	limitedReader := io.LimitReader(reader, maxBinarySize)
	_, copyErr := io.Copy(outFile, limitedReader)
	closeErr := outFile.Close()

	if copyErr != nil {
		return "", fmt.Errorf("could not write binary: %w", copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("could not close binary file: %w", closeErr)
	}

	return destPath, nil
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	upgradeCmd := newUpgradeCmd()
	upgradeCmd.GroupID = "config"
	rootCmd.AddCommand(upgradeCmd)
}

// copyFile copies a file from src to dst with the given permissions
func copyFile(src, dst string, perm os.FileMode) error {
	srcFile, err := os.Open(src) //nolint:gosec // Path is in controlled temp directory
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm) //nolint:gosec // Permissions set by caller
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
