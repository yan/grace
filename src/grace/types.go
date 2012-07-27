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

import "syscall"
import "os"

// Process represents a currently-executing process
type Process struct {
  Pid             int
  // Filename should be set to the binary backing the executable
  Filename        string
  // DebugSymbols is the symbol table in case the binary has debugging symbols
  // compiled in. If not, it's empty.
  DebugSymbols   *SymbolTable
  Memory          MemoryMap
  Files        []*os.File
  Breakpoints  []*Breakpoint
  Registers      *RegisterState

  isRunning       bool      
}

type RegisterState struct {
  syscall.PtraceRegs
}

const INT3 = 0xcc
type BpCallback func (*RegisterState) Action
type Breakpoint struct {
  Address    uint64
  savedInstr []byte
  Active     bool
  Callback   BpCallback
  HitCount   uint64
}

type TracerError string
type MemoryRegion struct {
  Address uint64
  Offset uint64
  File string
  Size int
  Permissions string // TODO: proper bitmask?
}

type MemoryMap map[uint64]MemoryRegion
type CompiledFile struct {
  Filename string
  Lowpc, Highpc  uint64
  Functions map[string]CompiledFunction
}

type SymbolTable map[string]CompiledFile

type CompiledFunction struct {
  Name string
  Lowpc, Highpc uint64
  Lineno  int
}
func (c CompiledFunction) Address() uint64 {
  return c.Lowpc
}

type InstantiatedRange interface {
  High() uint64
  Low() uint64
}

func (c CompiledFile) High() uint64 {
  return c.Highpc
}
func (c CompiledFile) Low() uint64 {
  return c.Lowpc
}
func (c CompiledFunction) High() uint64 {
  return c.Highpc
}
func (c CompiledFunction) Low() uint64 {
  return c.Lowpc
}

type Action int
const (
  CONTINUE = iota
  ABORT
)
