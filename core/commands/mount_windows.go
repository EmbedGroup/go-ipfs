package commands

import (
	"errors"

	cmds "gx/ipfs/QmRRovo1DE6i5cMjCbf19mQCSuszF6SKwdZNUMS7MtBnH1/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var MountCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Not yet implemented on Windows.",
		ShortDescription: "Not yet implemented on Windows. :(",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		res.SetError(errors.New("Mount isn't compatible with Windows yet"), cmdkit.ErrNormal)
	},
}
