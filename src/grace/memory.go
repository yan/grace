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
  "strings"
  "strconv"
  "os"
  "fmt"
  "bufio"
  "io"
)

const (
  Anonymous = iota
  File
)


func (m MemoryMap) findAddress(addr uint64) *MemoryRegion {
  for a, m := range m {
    if addr > a && addr < a + uint64(m.Size) {
      return &m
    }
  }
  return nil
}

func parseMemoryRegion(mapping string) (uint64, MemoryRegion) {
  fields := strings.Fields(mapping)

  // Location
  addrs_str := strings.Split(fields[0], "-")
  addr_start_ui, _ := strconv.ParseUint(addrs_str[0], 16, 64)
  addr_end_ui, _   := strconv.ParseUint(addrs_str[1], 16, 64)
  addr_start, addr_end := uint64(addr_start_ui), uint64(addr_end_ui)
  
  // Permissions
  perms := fields[1]

  // Offset into file
  offset_ui, _ := strconv.ParseUint(fields[2], 16, 64)
  offset := uint64(offset_ui)

  // ignore dev/inode
  file := ""
  if len(fields) > 5 {
    file = fields[5]
  }

  return addr_start, MemoryRegion { addr_start, offset, file,
                        int(addr_end-addr_start), perms, }

}

func getMemoryMap(pid int) (MemoryMap, error) {
  maps, err := os.Open(fmt.Sprintf("/proc/%d/maps", pid))
  if err != nil {
    return nil, err
  } 
  defer maps.Close()

  memoryMap := make(MemoryMap)
  mapreader := bufio.NewReader(maps)
  for {
    line, err := mapreader.ReadString('\n') 
    if err == io.EOF {
      break
    }
    addr, entry := parseMemoryRegion(line[:len(line)-1])
    memoryMap[addr] = entry
  }

  return memoryMap, err
}
