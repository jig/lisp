package debugimpl

import (
	"strings"

	"github.com/jig/lisp/debug"
	"github.com/jig/lisp/types"
)

type Instance struct {
	control     chan debug.DebugControl
	sendMessage func(debug.DebugMessage)
	doNotStop   bool
	exitNow     bool

	currentFileName string
	sourceFiles     map[string]string
}

func New(sendMessage func(debug.DebugMessage), ctrl chan debug.DebugControl) *Instance {
	return &Instance{
		sendMessage:     sendMessage,
		control:         ctrl,
		doNotStop:       false,
		exitNow:         false,
		currentFileName: "",
		sourceFiles:     make(map[string]string),
	}
}

// Wait receives a DebugMessage from Eval and sends it to the configured
// handler, then waits for a DebugControl command from the control channel to
// restart evaluation.
func (d *Instance) Wait(msg debug.DebugMessage) debug.DebugControl {
	// check whether special case when we are returning from a load-file call;
	// if that were the case we need to update the current file name and
	// contents back to the caller so debugger can keep track of the current file.
	if msg.Input != nil {
		pos := types.Pos(msg.Input)
		if d.currentFileName != pos.File() {
			setFilename := pos.File()
			setContents, ok := d.sourceFiles[pos.File()]
			if !ok {
				var fileKeys []string
				for key := range d.sourceFiles {
					fileKeys = append(fileKeys, key)
				}
				panic("Using a file never loaded is impossible: " + setFilename + " available files: " + strings.Join(fileKeys, ", "))
			}
			d.sendMessage(debug.DebugMessage{
				Filename: &setFilename,
				Contents: &setContents,
			})
		}
	}

	// Wait main duty starts here: send current state from Eval to the debugger configured handler.
	d.sendMessage(msg)

	// and then wait from the control channel for a command.
	ctrl := <-d.control

	// deal with special control commonds here and appropiately change the state of the debugger...
	switch ctrl {
	case debug.DebugContinue:
		d.doNotStop = true
	case debug.DebugExit:
		d.exitNow = true
	}

	// ...and return the control command.
	return ctrl
}

func (d *Instance) DoNotStopStatus() bool {
	return d.doNotStop
}

func (d *Instance) CancelStatus() bool {
	return d.exitNow
}

func (d *Instance) SetDoNotStop(i bool) (previously bool) {
	previously = d.doNotStop
	d.doNotStop = i
	return previously
}

func (d *Instance) SetCancelStatus(i bool) (previously bool) {
	previously = d.exitNow
	d.exitNow = i
	return previously
}

func (d *Instance) Reset() {
	d.doNotStop = false
	d.exitNow = false
}

func (d *Instance) PushFile(filename, contents string) {
	currentContents, exists := d.sourceFiles[filename]
	if exists {
		if currentContents == contents {
			return
		}
		panic("Attempting to overwrite existing file contents: " + filename)
	}

	d.sourceFiles[filename] = contents
	if d.sendMessage != nil {
		d.sendMessage(debug.DebugMessage{
			Filename: &filename,
			Contents: &contents,
		})
	}
}
