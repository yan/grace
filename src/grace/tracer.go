/*  Copyright (c) 2012 Yan Ivnitskiy. All rights reserved.
 *  
 *  Redistribution and use in source and binary forms, with or without
 *  modification, are permitted provided that the following conditions are
 *  met:
 *  
 *     * Redistributions of source code must retain the above copyright
 *  notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above
 *  copyright notice, this list of conditions and the following disclaimer
 *  in the documentation and/or other materials provided with the
 *  distribution.
 *     * Neither the name of grace nor the names of its
 *  contributors may be used to endorse or promote products derived from
 *  this software without specific prior written permission.
 *  
 *  THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 *  "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 *  LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 *  A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 *  OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 *  SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 *  LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 *  DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 *  THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 *  (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 *  OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package grace

import "os"
import "syscall"

func (t TracerError) Error() string {
  return string(t)
}

func (p *Process) FindTextSection() uint64 {
  for _, v := range p.Memory {
    if v.File == p.Filename && v.Permissions == "r-xp" {
      return v.Address
    }
  }
  return 0
}

func (p *Process) Continue() error {
  err := syscall.PtraceCont(p.Pid, 0)
  if err == nil {
    p.isRunning = true
  }
  return err
}

func (p *Process) AddInstrument(where string, callback BpCallback) bool{
  return true
}

func (p *Process) InBreakpoint() (*Breakpoint, bool) {
  regs, err := p.GetRegisters()
  if err != nil {
    return nil, false
  }

  pc := regs.PC()

  for _, bp := range p.Breakpoints {
    if bp.Address+1 == pc {
      return bp, true
    }
  }
  return nil, false
}

// StartProcess kicks off the event loop and forever waits for signals from
// the traced process. This is currently done in a super-silly fashion and will
// hopefully benefit from Go channels/goroutines in the future.
func (p *Process) StartProcess() (ret int) {
  var status syscall.WaitStatus

  L: for {
    _, err := syscall.Wait4(/*p.Pid*/-1, &status, 0, nil)
    p.isRunning = false

    switch {
    // status == 0  means terminated??
    case status.Exited() || status == 0 || err != nil:
      ret = status.ExitStatus()
      break L
    case status.Stopped():
      if bp, hit := p.InBreakpoint(); hit {
        p.handleBreakpoint(bp)
      }

    //case status.Continued():
    //case status.CoreDump():
    //case status.Signaled():
    //case status.ExitStatus():
    //case status.StopSignal():
    //case status.TrapCause():
    default:
      // fmt.Printf("Got status: %v\n", status)
    }

    p.Continue()

  }
  return
}

/* ----- public interface ----------- */
func Attach(pid int) (proc *Process, err error) {
  return nil, nil
}

// LoadExecutable opens binaryName, passing it args and attempts to exec it.
// Note, the process does not begin executing main() until after StartExecutable
// is called.
func LoadExecutable(binaryName string, args []string) (proc *Process, err error) {
	if _, ok := os.Stat(binaryName); ok != nil {
		proc, err = nil, &os.PathError{"LoadExecutable", binaryName, ok}
		return
	}

  var started *os.Process
	attr := &os.ProcAttr{
    Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
    Sys: &syscall.SysProcAttr{ Ptrace: true, },
  }
	if p, ok := os.StartProcess(binaryName, args, attr); ok != nil {
		proc, err = nil, &os.PathError{"LoadExecutable", binaryName, ok}
		return
	} else {
    started = p
  }

  proc = new(Process)
  proc.Pid = started.Pid
  proc.Memory, _ = getMemoryMap(proc.Pid)
  proc.Filename = binaryName
  proc.DebugSymbols, err = ExtractSymbolTable(binaryName, 0)
  proc.Breakpoints = []*Breakpoint{}

	return
}

