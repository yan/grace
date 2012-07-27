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

import (
  "syscall"
  "unsafe"
  "os"
  "fmt"
)

// SetRegisters is a wrapper for ptrace(PTRACE_SETREGS)
func (p *Process) SetRegisters(regs *RegisterState) bool {
  err := syscall.PtraceSetRegs(p.Pid, &regs.PtraceRegs)
  if err != nil {
    return false
  }
  return true
}

// GetRegisters is a wrapper for ptrace(PTRACE_GETREGS)
func (p *Process) GetRegisters() (*RegisterState, error) {
  registers := &RegisterState{}
  err := syscall.PtraceGetRegs(p.Pid, &registers.PtraceRegs)
  if err != nil {
    return nil, err
  }
  return registers, nil
}

// ensureNotRunning panics when the current process that is being traced is
// actually executing. Used when setting/removing breakpoints and otherwise
// modifying the target process.
func (p *Process) ensureNotRunning() {
  if (p.isRunning) {
    panic("Running when it shouldn't be!")
  }
}

// writeMemoryAligned is a wrapper for ptrace(PTRACE_POKETEXT) that attempts to
// make all calls wordsize-aligned. For some reason, this is completely
// different from readMemoryAligned and they should be merged.
func (p *Process) writeMemoryAligned(where uint64, bytes []byte) (count int, err error) {
  wordsize := int(unsafe.Sizeof(uintptr(0)))
  rem := len(bytes) % wordsize

  if rem > 0 {
    pad := make([]byte, wordsize)
    syscall.PtracePokeText(p.Pid, uintptr(where+uint64(len(bytes)-rem)), pad)
    bytes = append(bytes, pad[rem:]...)
  }

  for offset := 0; offset < len(bytes); offset += wordsize {
    toWrite := bytes[offset:offset+wordsize]
    cnt, err := syscall.PtracePokeText(p.Pid, uintptr(where+uint64(offset)), toWrite)
    if err != nil {
      return cnt, err
    }
    count += cnt
  }

  return
}


// readMemoryAligned is a wrapper for ptrace(PTRACE_PEEKTEXT) that attempts to
// make all calls wordsize-aligned. (See comment from writeMemoryAligned)
func (p *Process) readMemoryAligned(where uint64, bytes []byte) (count int, err error) {
  p.ensureNotRunning()

  wordSize := int(unsafe.Sizeof(uintptr(0)))
  whereAligned := where & uint64(^(wordSize-1))
  firstWordOffset := int(where - whereAligned)
  lenBytes := len(bytes)
  total := 0

  wordRead := make([]byte, wordSize)
  for i := 0; i < lenBytes; {
    cnt, err := syscall.PtracePeekText(p.Pid, uintptr(whereAligned), wordRead)

    if err != nil || cnt != wordSize {
      panic(err.Error())
      return total, err
    }

    i += copy(bytes[i:lenBytes], wordRead[firstWordOffset:])

    firstWordOffset = 0
    total = i
  }

  return total, nil
}

// SwapBytesText simple writes the slice 'what' to the location 'where' in the
// target process, returning the content that used to be at that address in
// the 'what' slice.
func (p *Process) SwapBytesText(where uint64, what []byte) bool {
  p.ensureNotRunning()

  saved := make([]byte, len(what))
  //cnt, err := syscall.PtracePeekText(p.Pid, uintptr(where), saved)
  cnt, err := p.readMemoryAligned(where, saved)
  if cnt != len(what) || err != nil {
    return false
  }

  cnt, err = syscall.PtracePokeText(p.Pid, uintptr(where), what)
  if cnt != len(what) || err != nil {
    fmt.Printf("failed writing")
    return false
  }

  copy(what, saved)
  return true
}

type PtraceError string
func (p PtraceError) Error() string {
  return string(p)
}

// ToggleBreakpoint should be clear, but needs more work.
// TODO: this function is very asymetrical with Deactivate..
func (p *Process) ToggleBreakpoint(bp *Breakpoint) bool {
  if ! p.SwapBytesText(bp.Address, bp.savedInstr) {
    return false
  }
  bp.Active = !bp.Active
  return true
}

// handleBreakpoint gets called when the event loop gets a signal from the
// traced process.
func (proc *Process) handleBreakpoint(bp *Breakpoint) {
  regs, err := proc.GetRegisters()
  if err != nil {
    return
  }

  // restore original instruction
  /*ok := */ proc.SwapBytesText(bp.Address, bp.savedInstr)
  /*
    if ! ok {
      fmt.Printf("!!! NO1 !!!\n")
    }
  */

  // restore original instruction
  regs.SetPC(bp.Address)
  proc.SetRegisters(regs)

  // single step
  proc.SingleStep()

  // Invoke the callback
  switch result := bp.Callback(regs); result {
    case ABORT: os.Exit(0) // TODO: Not very graceful
    case CONTINUE:
  }

  // restore again
  proc.SwapBytesText(bp.Address, bp.savedInstr)

  bp.HitCount = bp.HitCount + 1
}

// SingleStep is a wrapper for ptrace(PTRACE_STEP)
func (p *Process) SingleStep() bool {
  err := syscall.PtraceSingleStep(p.Pid)
  return err == nil
}


// AddBreakpoint installs an INT3 (or otherwise set instruction sequence) at
// the address 'where' and registers 'fun' as the callback to be invoked every
// time it's hit.
func (p *Process) AddBreakpoint(where string, fun BpCallback) bool {
  address, err := p.resolveSymbol(where)
  if err != nil {
    return false
  }

  // TODO: make the bp instruction/instruction sequence settable by the user
  savedInstr := []byte{INT3}
  if ok := p.SwapBytesText(address, savedInstr); ok {
    p.Breakpoints = append(p.Breakpoints, &Breakpoint{address, savedInstr,
                                                      true, fun, 0})
    return true
  }
  return false
}

// Kill sends SIGKILL to the target process, in a currently roundabout way.
func (p *Process) Kill() {
  // TODO: Clean up this hack
  proc, _ := os.FindProcess(p.Pid)
  proc.Kill()
}

