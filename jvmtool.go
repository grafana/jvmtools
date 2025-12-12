package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"

	"github.com/grafana/jvmtools/jvm"
)

func validCommand(arg string) bool {
	validCmds := map[string]struct{}{
		"load":            {},
		"threaddump":      {},
		"dumpheap":        {},
		"setflag":         {},
		"properties":      {},
		"jcmd":            {},
		"inspectheap":     {},
		"datadump":        {},
		"printflag":       {},
		"agentProperties": {},
	}

	_, ok := validCmds[arg]
	return ok
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	logger := slog.With("component", "jvmtool")

	if len(os.Args) < 3 {
		fmt.Println("Usage: jvmtool <pid> <cmd> [args ...]")
		fmt.Println("Commands:")
		fmt.Println("    load  threaddump   dumpheap  setflag    properties")
		fmt.Println("    jcmd  inspectheap  datadump  printflag  agentProperties")
		os.Exit(1)
	}

	pid, err := strconv.Atoi(os.Args[1])
	if err != nil || pid <= 0 {
		fmt.Fprintf(os.Stderr, "%s is not a valid process ID\n", os.Args[1])
		os.Exit(1)
	}

	if ok := validCommand(os.Args[2]); !ok {
		fmt.Printf("%v is not a valid jvmtool command\n", os.Args[2])
		fmt.Println("Valid Commands:")
		fmt.Println("    load  threaddump   dumpheap  setflag    properties")
		fmt.Println("    jcmd  inspectheap  datadump  printflag  agentProperties")
		os.Exit(1)
	}

	// status, err := jvm.EnableDynamicAgentLoading(pid)

	// if err != nil {
	// 	logger.Error("encountered error while enabling dynamic loading", "error", err)
	// } else {
	// 	logger.Info("dynamic loading status", "result", status)
	// }

	attacher := jvm.NewJAttacher(logger)
	attacher.Init()

	out, err := attacher.Attach(pid, os.Args[2:])
	if err != nil {
		logger.Error("encountered error while executing jattach", "error", err)
		attacher.Cleanup()
		os.Exit(1)
	}

	// use bufio.Scanner for more insights about the output
	reader := bufio.NewReader(out)
	buf := bytes.Buffer{}
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF { // hotspot terminates with EOF
				// Process the last line if it doesn't end with newline
				break
			}
			logger.Error("error reading line", "error", err)
			os.Exit(2)
		}

		buf.WriteByte(b)
		if b == '\n' {
			os.Stdout.Write(buf.Bytes())
			buf.Reset()
		} else if b == 0 { // j9 terminates with 0
			os.Stdout.Write(buf.Bytes())
			break
		}
	}

	out.Close()
	attacher.Cleanup()

	os.Exit(0)
}
