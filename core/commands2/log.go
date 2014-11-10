package commands

import (
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	u "github.com/jbenet/go-ipfs/util"
)

var logCmd = &cmds.Command{
	Description: "Change the logging level",
	Help: `'ipfs log' is a utility command used to change the logging
output of a running daemon.
`,

	Arguments: []cmds.Argument{
		cmds.Argument{"subsystem", cmds.ArgString, true, false,
			"the subsystem logging identifier. Use * for all subsystems."},
		cmds.Argument{"level", cmds.ArgString, true, false,
			"one of: debug, info, notice, warning, error, critical"},
	},
	Run: func(res cmds.Response, req cmds.Request) {
		args := req.Arguments()
		if err := u.SetLogLevel(args[0].(string), args[1].(string)); err != nil {
			res.SetError(err, cmds.ErrClient)
			return
		}

		s := fmt.Sprintf("Changed log level of '%s' to '%s'", args[0], args[1])
		log.Info(s)
		res.SetOutput(&MessageOutput{s})
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: MessageTextMarshaller,
	},
	Type: &MessageOutput{},
}
