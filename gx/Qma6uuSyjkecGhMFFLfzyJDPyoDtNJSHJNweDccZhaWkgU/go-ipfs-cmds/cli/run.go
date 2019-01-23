package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	cmds "mbfs/go-mbfs/gx/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"mbfs/go-mbfs/gx/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

// ExitError is the error used when a specific exit code needs to be returned.
type ExitError int

func (e ExitError) Error() string {
	return fmt.Sprintf("exit code %d", int(e))
}

// Closer is a helper interface to check if the env supports closing
type Closer interface {
	Close()
}

func Run(ctx context.Context, root *cmds.Command,
	cmdline []string, stdin, stdout, stderr *os.File,
	buildEnv cmds.MakeEnvironment, makeExecutor cmds.MakeExecutor) error {

	//报错函数供后续调用
	printErr := func(err error) {
		fmt.Fprintf(stderr, "Error: %s\n", err)
	}

	//将ctx,命令行参数和root（包含所有subcommand）转化为request结构体
	req, errParse := Parse(ctx, cmdline[1:], stdin, root)

	//设置超时时间
	var cancel func()
	if timeoutStr, ok := req.Options[cmds.TimeoutOpt]; ok {
		timeout, err := time.ParseDuration(timeoutStr.(string))
		if err != nil {
			printErr(err)
			return err
		}
		req.Context, cancel = context.WithTimeout(req.Context, timeout)
	} else {
		req.Context, cancel = context.WithCancel(req.Context)
	}
	defer cancel()

	// this is a message to tell the user how to get the help text
	printMetaHelp := func(w io.Writer) {
		cmdPath := strings.Join(req.Path, " ")
		fmt.Fprintf(w, "Use '%s %s --help' for information about this command\n", cmdline[0], cmdPath)
	}
	// 定义打印函数
	printHelp := func(long bool, w io.Writer) {
		helpFunc := ShortHelp
		if long {
			helpFunc = LongHelp
		}

		var path []string
		if req != nil {
			path = req.Path
		}

		if err := helpFunc(cmdline[0], root, path, w); err != nil {
			// This should not happen
			panic(err)
		}
	}

	// BEFORE handling the parse error, if we have enough information
	// AND the user requested help, print it out and exit
	// 如果命令行是help，则打印返回，否则err=ErrNoHelpRequested，并继续
	err := HandleHelp(cmdline[0], req, stdout)
	if err == nil {
		return nil
	} else if err != ErrNoHelpRequested {
		return err
	}
	// no help requested, continue.

	// ok now handle parse error (which means cli input was wrong,
	// e.g. incorrect number of args, or nonexistent subcommand)
	// 现在处理上头的参数解析错误
	if errParse != nil {
		printErr(errParse)

		// 用户使用错误，直接报错退出
		if req != nil && req.Command != nil {
			fmt.Fprintln(stderr) // i need some space
			printHelp(false, stderr)
		}

		return err
	}

	// here we handle the cases where
	// - commands with no Run func are invoked directly.
	// - the main command is invoked.
	// 以下是代码错误：代码未实现相应的子命令函数，打印到标准输出
	if req == nil || req.Command == nil || req.Command.Run == nil {
		printHelp(false, stdout)
		return nil
	}

	// 此时的cmd已经是子命令对用的command结构体
	cmd := req.Command

	// 构建环境信息，这个函数会返回包含配置和IpfsNode构造函数的结构体
	env, err := buildEnv(req.Context, req)
	if err != nil {
		printErr(err)
		return err
	}
	if c, ok := env.(Closer); ok {
		defer c.Close()
	}

	// 这个函数经过一些步骤之后，找到最终要执行的函数
	exctr, err := makeExecutor(req, env)
	if err != nil {
		printErr(err)
		return err
	}

	var (
		re     cmds.ResponseEmitter
		exitCh <-chan int
	)

	encTypeStr, _ := req.Options[cmds.EncLong].(string)
	encType := cmds.EncodingType(encTypeStr)

	// use JSON if text was requested but the command doesn't have a text-encoder
	// 如果子命令没有实现对应文本解析器，使用json
	if _, ok := cmd.Encoders[encType]; encType == cmds.Text && !ok {
		req.Options[cmds.EncLong] = cmds.JSON
	}

	// first if condition checks the command's encoder map, second checks global encoder map (cmd vs. cmds)
	// 生成responseEmitter
	re, exitCh, err = NewResponseEmitter(stdout, stderr, req)
	if err != nil {
		printErr(err)
		return err
	}

	//执行子命令的Run函数，跳到真正执行函数的地方
	errCh := make(chan error, 1)
	go func() {
		err := exctr.Execute(req, re, env)
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		printErr(err)

		if kiterr, ok := err.(*cmdkit.Error); ok {
			err = *kiterr
		}
		if kiterr, ok := err.(cmdkit.Error); ok && kiterr.Code == cmdkit.ErrClient {
			printMetaHelp(stderr)
		}

		return err

	case code := <-exitCh:
		if code != 0 {
			return ExitError(code)
		}
	}

	return nil
}
