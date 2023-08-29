package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"golang.org/x/crypto/ssh"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

type TartInfo struct {
	ECID                string
	HardwareModelBase64 string
}

func main() {
	vmName := "ventura-base"
	outputFile := "data.cvs"

	args := os.Args[1:]
	if len(args) > 1 {
		outputFile = args[1]
	}
	if len(args) > 0 {
		vmName = args[0]
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	tartBinaryPath := path.Join(homeDir, "workspace", "tart", ".build", "debug", "tart")

	f, err := os.Create(outputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	for i := 0; i < 50_000; i++ {
		info, err := collectForVM(context.Background(), tartBinaryPath, vmName)
		if err != nil {
			panic(err)
		}
		line := fmt.Sprintf("%s\t%s", info.ECID, info.HardwareModelBase64)
		println(line)
		_, _ = f.WriteString(line + "\n")
	}
}

func collectForVM(ctx context.Context, tartBinaryPath, vmName string) (*TartInfo, error) {
	runCmd := exec.CommandContext(ctx, tartBinaryPath, "run", "--no-graphics", vmName)
	runCmd.Env = runCmd.Environ()

	var runOut, runErr bytes.Buffer
	runCmd.Stdout = &runOut
	runCmd.Stderr = &runErr

	err := runCmd.Start()
	if err != nil {
		return nil, err
	}

	time.Sleep(15 * time.Second)

	ipCmd := exec.CommandContext(ctx, tartBinaryPath, "ip", "--wait", "60", vmName)
	ipCmd.Env = runCmd.Environ()

	var ipOut, ipErr bytes.Buffer
	ipCmd.Stdout = &ipOut
	ipCmd.Stderr = &ipErr

	err = ipCmd.Run()
	if err != nil {
		return nil, err
	}

	ip := firstNonEmptyLine(ipOut.String(), ipErr.String())

	if ip == "" {
		return nil, fmt.Errorf("failed to get ip")
	}

	serrianNumber, err := sshAndGetSerialNumber(ctx, ip)
	err = runCmd.Wait()
	if err != nil {
		return nil, err
	}
	hardwareModelBase64 := firstNonEmptyLine(runOut.String())
	return &TartInfo{
		ECID:                serrianNumber,
		HardwareModelBase64: hardwareModelBase64,
	}, nil
}

func sshAndGetSerialNumber(ctx context.Context, ip string) (string, error) {
	var netConn net.Conn
	var err error

	addr := ip + ":22"

	if err := retry.Do(func() error {
		dialer := net.Dialer{}

		netConn, err = dialer.DialContext(ctx, "tcp", addr)

		return err
	}, retry.Context(ctx)); err != nil {
		return "", fmt.Errorf("failed to connect via SSH: %v", err)
	}

	sshConfig := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		User: "admin",
		Auth: []ssh.AuthMethod{
			ssh.Password("admin"),
		},
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, sshConfig)
	if err != nil {
		return "", fmt.Errorf("failed to connect via SSH: %v", err)
	}

	cli := ssh.NewClient(sshConn, chans, reqs)

	sess, err := cli.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to open SSH session: %v", err)
	}

	// Log output from the virtual machine
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("%w: while opening stdout pipe: %v", err)
	}

	stdinBuf, err := sess.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("while opening stdin pipe: %v", err)
	}

	// start a login shell so all the customization from ~/.zprofile will be picked up
	err = sess.Shell()
	if err != nil {
		return "", fmt.Errorf("failed to start a shell: %v", err)
	}

	_, err = stdinBuf.Write([]byte("ioreg -l\nsudo shutdown -h now\n"))

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		for _, line := range strings.Split(scanner.Text(), "\n") {
			if strings.Contains(line, "IOPlatformSerialNumber") {
				parts := strings.Split(line, "\"")
				return parts[len(parts)-1-1], nil
			}
		}
	}

	return "", nil
}

func firstNonEmptyLine(outputs ...string) string {
	for _, output := range outputs {
		for _, line := range strings.Split(output, "\n") {
			if line != "" {
				return line
			}
		}
	}

	return ""
}
