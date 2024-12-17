package jvm

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/grafana/jvmtools/util"
)

// Check if remote JVM has already opened socket for Dynamic Attach
func checkSocket(pid int, tmpPath string) bool {
	path := fmt.Sprintf("%s/.java_pid%d", tmpPath, pid)
	info, err := os.Stat(path)
	return err == nil && (info.Mode()&os.ModeSocket != 0)
}

// Check if a file is owned by current user
func getFileOwner(path string) (uid int) {
	info, err := os.Stat(path)
	if err != nil {
		return -1
	}
	stat := info.Sys().(*syscall.Stat_t)
	return int(stat.Uid)
}

// Force remote JVM to start Attach listener.
// HotSpot will start Attach listener in response to SIGQUIT if it sees .attach_pid file
func startAttachMechanism(pid, nspid, attachPid int, tmpPath string) bool {
	path := fmt.Sprintf("/proc/%d/cwd/.attach_pid%d", attachPid, nspid)
	fd, err := os.Create(path)
	if err != nil || (fd.Close() == nil && getFileOwner(path) != os.Geteuid()) {
		os.Remove(path)
		path = fmt.Sprintf("%s/.attach_pid%d", tmpPath, nspid)
		fd, err = os.Create(path)
		if err != nil {
			return false
		}
		fd.Close()
	}

	syscall.Kill(pid, syscall.SIGQUIT)

	ts := 20 * time.Millisecond
	for i := 0; i < 300; i++ {
		time.Sleep(ts)
		if checkSocket(nspid, tmpPath) {
			os.Remove(path)
			return true
		}
		ts += 20 * time.Millisecond
	}

	os.Remove(path)
	return false
}

// Connect to UNIX domain socket created by JVM for Dynamic Attach
func connectSocket(pid int, tmpPath string) (int, error) {
	addr := &syscall.SockaddrUnix{Name: fmt.Sprintf("%s/.java_pid%d", tmpPath, pid)}
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return -1, err
	}
	if err := syscall.Connect(fd, addr); err != nil {
		syscall.Close(fd)
		return -1, err
	}
	return fd, nil
}

// Send command with arguments to socket
func writeCommand(fd int, args []string) error {
	request := make([]byte, 0)

	request = append(request, byte('1'))
	request = append(request, byte(0))

	for i := 0; i < 4; i++ {
		if i < len(args) {
			request = append(request, []byte(args[i])...)
		}
		request = append(request, byte(0))
	}

	_, err := syscall.Write(fd, request)
	return err
}

// Mirror response from remote JVM to stdout
func readResponse(fd int, args []string, out chan []byte, logger *slog.Logger) int {
	buf := make([]byte, 8192)
	n, err := syscall.Read(fd, buf)
	if err != nil {
		logger.Error("error reading response from JVM", "error", err)
		return 1
	}
	if n == 0 {
		logger.Error("unexpected EOF while reading response from the JVM")
		return 1
	}

	buf = buf[:n]
	nlPos := bytes.IndexByte(buf, '\n')
	if nlPos < 0 {
		nlPos = n
	}

	result, _ := strconv.Atoi(string(buf[:nlPos]))

	if len(args) > 0 && args[0] == "load" {
		total := n
		for total < len(buf)-1 {
			n, err = syscall.Read(fd, buf[total:])
			if err != nil || n == 0 {
				break
			}
			total += n
		}
		buf = buf[:total]

		if result == 0 && len(buf) >= 2 {
			if strings.HasPrefix(string(buf[2:]), "return code: ") {
				result, _ = strconv.Atoi(string(buf[15:]))
			} else if (buf[2] >= '0' && buf[2] <= '9') || buf[2] == '-' {
				result, _ = strconv.Atoi(string(buf[2:]))
			} else {
				result = -1
			}
		}
	}

	logger.Info("JVM response", "code", result)

	if nlPos < n-1 {
		out <- buf[nlPos+1:]
	}

	for {
		n, err := syscall.Read(fd, buf)
		if n == 0 || err != nil {
			break
		}
		out <- buf[:n]
	}

	out <- []byte(fmt.Sprintln())

	return result
}

func jattachHotspot(pid, nspid, attachPid int, args []string, tmpPath string, out chan []byte, logger *slog.Logger) int {
	if !checkSocket(nspid, tmpPath) && !startAttachMechanism(pid, nspid, attachPid, tmpPath) {
		logger.Error("could not start the attach mechanism")
		return 1
	}

	fd, err := connectSocket(nspid, tmpPath)
	if err != nil {
		logger.Error("could not connect to JVM socket", "error", err)
		return 1
	}
	defer syscall.Close(fd)

	logger.Info("connected to the JVM")

	if err := writeCommand(fd, args); err != nil {
		logger.Error("error writing to the JVM socket", "error", err)
		return 1
	}

	return readResponse(fd, args, out, logger)
}

func Jattach(pid int, argv []string, out chan []byte, logger *slog.Logger) int {
	myUID := syscall.Geteuid()
	myGID := syscall.Getegid()
	targetUID := myUID
	targetGID := myGID
	var nspid int

	if util.GetProcessInfo(pid, &targetUID, &targetGID, &nspid) != nil {
		logger.Error("process not found", "pid", pid)
		return 1
	}

	// Container support: switch to the target namespaces.
	// Network and IPC namespaces are essential for OpenJ9 connection.
	util.EnterNS(pid, "net")
	util.EnterNS(pid, "ipc")
	mntChanged := util.EnterNS(pid, "mnt")

	// In HotSpot, dynamic attach is allowed only for the clients with the same euid/egid.
	// If we are running under root, switch to the required euid/egid automatically.
	if (myGID != targetGID && syscall.Setegid(int(targetGID)) != nil) ||
		(myUID != targetUID && syscall.Seteuid(int(targetUID)) != nil) {
		logger.Error("failed to change credentials to match the target process")
		return 1
	}

	attachPid := pid
	if mntChanged > 0 {
		attachPid = nspid
	}

	tmpPath := util.GetTmpPath(attachPid)

	// Make write() return EPIPE instead of abnormal process termination
	signal.Ignore(syscall.SIGPIPE)

	res := jattachHotspot(pid, nspid, attachPid, argv, tmpPath, out, logger)
	return res
}
