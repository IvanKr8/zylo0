package client

import (
	"flag"
	"fmt"
	"os"
)

func buildDownCommand(args []string, tty string) Command {
	fs := flag.NewFlagSet("down", flag.ExitOnError)
	hash := fs.String("hash", "", "container hash")
	fs.Parse(args)

	return Command{
		Op:   "DOWN",
		TTY:  tty,
		Path: getCwd(),
		Data: map[string]string{"hash": *hash},
	}
}

func buildLogsCommand(args []string, tty string) Command {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	hash := fs.String("hash", "", "container hash")
	logType := fs.String("type", "output", "log type (output|daemon)")
	fs.Parse(args)

	if *logType != "output" && *logType != "daemon" {
		fmt.Println("Error: type must be output or daemon")
		os.Exit(1)
	}

	return Command{
		Op:   "LOGS",
		TTY:  tty,
		Path: getCwd(),
		Data: map[string]string{
			"hash": *hash,
			"type": *logType,
		},
	}
}

func buildExecCommand(args []string, tty string) Command {
	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	user := fs.String("user", "", "run as user")
	workdir := fs.String("workdir", "", "working directory")
	interactive := fs.Bool("i", false, "interactive mode")
	ttyFlag := fs.Bool("t", false, "allocate TTY")
	fs.Parse(args)

	if fs.NArg() < 2 {
		fmt.Println("Usage: zylo exec [options] CONTAINER COMMAND [ARG...]")
		os.Exit(1)
	}

	return Command{
		Op:  "EXEC",
		TTY: tty,
		Data: map[string]interface{}{
			"container":   fs.Arg(0),
			"command":     fs.Args()[1:],
			"user":        *user,
			"workdir":     *workdir,
			"interactive": *interactive || *ttyFlag,
			"tty":         *ttyFlag,
		},
	}
}

func buildVolumeCommand(args []string, tty string) Command {
	if len(args) < 1 {
		fmt.Println("Usage: zylo volume <list|delete>")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		return Command{Op: "VOLUME_LIST", TTY: tty}
	case "delete":
		return buildVolumeDeleteCommand(args[1:], tty)
	default:
		fmt.Printf("Unknown volume command: %s\n", args[0])
		os.Exit(1)
	}
	return Command{}
}

func buildVolumeDeleteCommand(args []string, tty string) Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	name := fs.String("name", "", "volume name")
	path := fs.String("path", "", "volume path")
	fs.Parse(args)

	if *name == "" && *path == "" {
		fmt.Println("Error: specify volume by --name or --path")
		os.Exit(1)
	}
	if *name != "" && *path != "" {
		fmt.Println("Error: use either --name or --path, not both")
		os.Exit(1)
	}

	return Command{
		Op:  "VOLUME_DELETE",
		TTY: tty,
		Data: map[string]string{
			"name": *name,
			"path": *path,
		},
	}
}

func buildImageCommand(args []string, tty string) Command {
	if len(args) < 1 {
		fmt.Println("Usage: zylo image <list|pull|delete>")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		return Command{Op: "IMAGE_LIST", TTY: tty}
	case "pull":
		return buildImagePullCommand(args[1:], tty)
	case "delete":
		return buildImageDeleteCommand(args[1:], tty)
	default:
		fmt.Printf("Unknown image command: %s\n", args[0])
		os.Exit(1)
	}
	return Command{}
}

func buildImagePullCommand(args []string, tty string) Command {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	name := fs.String("name", "", "image name")
	fs.Parse(args)

	if *name == "" {
		fmt.Println("Error: image name required (--name)")
		os.Exit(1)
	}

	return Command{
		Op:   "IMAGE_PULL",
		TTY:  tty,
		Data: map[string]string{"name": *name},
	}
}

func buildImageDeleteCommand(args []string, tty string) Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	name := fs.String("name", "", "image name")
	fs.Parse(args)

	if *name == "" {
		fmt.Println("Error: image name required (--name)")
		os.Exit(1)
	}

	return Command{
		Op:   "IMAGE_DELETE",
		TTY:  tty,
		Data: map[string]string{"name": *name},
	}
}
