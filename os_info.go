package main

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// AgentVersion is set at build time
var AgentVersion = "dev"

type OSInfo struct {
	Platform     string `json:"platform"`
	Distro       string `json:"distro"`
	Version      string `json:"version"`
	Manager      string `json:"manager"`
	Hostname     string `json:"hostname"`
	Arch         string `json:"arch"`
	AgentVersion string `json:"agent_version"`
	Uptime       int64  `json:"uptime_seconds"`
	IP           string `json:"ip_address"`
}

func GetOSInfo() OSInfo {
	hostname, _ := os.Hostname()

	info := OSInfo{
		Platform:     runtime.GOOS,
		Hostname:     hostname,
		Arch:         runtime.GOARCH,
		AgentVersion: AgentVersion,
		Uptime:       getUptime(),
		IP:           getOutboundIP(),
	}

	switch info.Platform {
	case "linux":
		info.Distro, info.Version = getLinuxDistroInfo()
		info.Manager = detectLinuxPackageManager(info.Distro)
	case "darwin":
		info.Distro = "macos"
		info.Version = getMacVersion()
		info.Manager = "brew" // Assume Homebrew on macOS
	case "windows":
		info.Distro = "windows"
		info.Version = getWindowsVersion()
		info.Manager = "winget" // Assume WinGet or Chocolatey
	default:
		info.Distro = "unknown"
		info.Version = "unknown"
		info.Manager = "unknown"
	}

	return info
}

func getUptime() int64 {
	// Simple cross-platform approximation or specific implementation
	// For now, return 0 if hard to get without CGO or complex parsing
	// Linux: /proc/uptime
	// Mac: sysctl kern.boottime (needs parsing)
	return 0
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func getWindowsVersion() string {
	// Basic implementation
	return "10/11"
}

func getLinuxDistroInfo() (string, string) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "linux", "unknown"
	}
	defer file.Close()

	var id, versionID string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}

	if id == "" {
		id = "linux"
	}
	return id, versionID
}

func detectLinuxPackageManager(distro string) string {
	switch distro {
	case "ubuntu", "debian", "pop", "mint":
		return "apt"
	case "fedora", "centos", "rhel":
		return "dnf" // or yum
	case "arch", "manjaro":
		return "pacman"
	case "alpine":
		return "apk"
	default:
		// Fallback detection by checking executable existence
		if _, err := exec.LookPath("apt-get"); err == nil {
			return "apt"
		}
		if _, err := exec.LookPath("dnf"); err == nil {
			return "dnf"
		}
		if _, err := exec.LookPath("pacman"); err == nil {
			return "pacman"
		}
		if _, err := exec.LookPath("yum"); err == nil {
			return "yum"
		}
		if _, err := exec.LookPath("apk"); err == nil {
			return "apk"
		}
		return "unknown"
	}
}

func getMacVersion() string {
	cmd := exec.Command("sw_vers", "-productVersion")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out.String())
}
